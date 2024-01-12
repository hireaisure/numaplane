package controller

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/numaproj-labs/numaplane/api/v1"
	"github.com/numaproj-labs/numaplane/internal/git"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testGitSyncName = "test-gitsync"
	testNamespace   = "test-ns"
)

var (
	defaultGitSync = &apiv1.GitSync{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testGitSyncName,
		},
		Spec: apiv1.GitSyncSpec{
			RepositoryPaths: []apiv1.RepositoryPath{
				{
					Name:           "my-controller",
					RepoUrl:        "https://github.com/myrepo.git",
					Path:           "./numaflowController/",
					TargetRevision: "main",
				},
				{
					Name:           "my-pipelines",
					RepoUrl:        "https://github.com/myrepo.git",
					Path:           "./pipelines/",
					TargetRevision: "main",
				},
			},
			Destinations: []apiv1.Destination{
				{
					Cluster:   "staging-usw2-k8s",
					Namespace: "team-a-namespace",
				},
				{
					Cluster:   "staging-use2-k8s",
					Namespace: "team-a-namespace",
				},
			},
		},
	}
)

func init() {
	_ = apiv1.AddToScheme(scheme.Scheme)
	_ = appv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
}

// test reconciliation of GitSync as it progresses through creation, update, and deletion
func Test_GitSyncLifecycle(t *testing.T) {
	t.Run("GitSync lifecycle", func(t *testing.T) {
		gitSync := defaultGitSync.DeepCopy()

		client := fake.NewClientBuilder().Build()
		err := os.Setenv("CLUSTER_NAME", "staging-usw2-k8s")
		assert.Nil(t, err)
		r, err := NewGitSyncReconciler(client, scheme.Scheme)
		assert.Nil(t, err)
		assert.NotNil(t, r)

		// reconcile the newly created GitSync
		reconcile(t, r, gitSync)
		verifyRunning(t, r, gitSync)

		// update the spec
		gitSync.Spec.RepositoryPaths[0].Path = gitSync.Spec.RepositoryPaths[0].Path + "xyz"
		reconcile(t, r, gitSync)
		verifyRunning(t, r, gitSync)

		// mark the GitSync for deletion
		now := metav1.Now()
		gitSync.DeletionTimestamp = &now
		reconcile(t, r, gitSync)
		verifyDeleted(t, r, gitSync)

	})

}

// Test the changing of destinations in the GitSync
// GitSync should be added to our GitSyncProcessor map if our cluster matches one of the clusters, but removed if it's not
func Test_GitSyncDestinationChanges(t *testing.T) {
	t.Run("GitSync destination test", func(t *testing.T) {
		gitSync := defaultGitSync.DeepCopy()
		gitSync.Spec.Destinations = []apiv1.Destination{ // doesn't include our cluster
			{
				Cluster:   "staging-use2-k8s",
				Namespace: "team-a-namespace",
			},
		}

		client := fake.NewClientBuilder().Build()
		err := os.Setenv("CLUSTER_NAME", "staging-usw2-k8s")
		assert.Nil(t, err)
		r, err := NewGitSyncReconciler(client, scheme.Scheme)
		assert.Nil(t, err)
		assert.NotNil(t, r)

		// our cluster is not one of the destinations, so it shouldn't end up in the map
		reconcile(t, r, gitSync)
		verifyNotApplicable(t, r, gitSync)

		// now update the spec so that it is one of the destinations
		gitSync = defaultGitSync.DeepCopy()
		reconcile(t, r, gitSync)
		verifyRunning(t, r, gitSync)
	})
}

func reconcile(t *testing.T, r *GitSyncReconciler, gitSync *apiv1.GitSync) {
	err := r.reconcile(context.Background(), gitSync)
	assert.NoError(t, err)
}

// check that a GitSync is Running
func verifyRunning(t *testing.T, r *GitSyncReconciler, gitSync *apiv1.GitSync) {
	// verify in map
	processorAsInterface, found := r.gitSyncProcessors.Load(gitSync.String())
	assert.True(t, found)
	assert.NotPanics(t, func() { _ = processorAsInterface.(*git.GitSyncProcessor) })

	// verify phase and Conditions
	assert.Equal(t, apiv1.GitSyncPhaseRunning, gitSync.Status.Phase)
	assert.Equal(t, string(apiv1.GitSyncConditionConfigured), gitSync.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, gitSync.Status.Conditions[0].Status)
}

// check that a GitSync is deemed Not-Applicable
func verifyNotApplicable(t *testing.T, r *GitSyncReconciler, gitSync *apiv1.GitSync) {
	// verify not in map
	_, found := r.gitSyncProcessors.Load(gitSync.String())
	assert.False(t, found)

	// verify phase and Conditions
	assert.Equal(t, apiv1.GitSyncPhaseNA, gitSync.Status.Phase)
	assert.Equal(t, string(apiv1.GitSyncConditionConfigured), gitSync.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, gitSync.Status.Conditions[0].Status)
}

// check that a GitSync is in a Deleted state
func verifyDeleted(t *testing.T, r *GitSyncReconciler, gitSync *apiv1.GitSync) {
	// verify not in map
	_, found := r.gitSyncProcessors.Load(gitSync.String())
	assert.False(t, found)
}