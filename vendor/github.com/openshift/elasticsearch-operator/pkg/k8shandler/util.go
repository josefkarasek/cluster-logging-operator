package k8shandler

import (
	"fmt"
	"os"

	api "github.com/openshift/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"

	v1alpha1 "github.com/openshift/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	defaultMasterCPULimit   = "100m"
	defaultMasterCPURequest = "100m"
	defaultCPULimit         = "4000m"
	defaultCPURequest       = "100m"
	defaultMemoryLimit      = "4Gi"
	defaultMemoryRequest    = "1Gi"
	MAX_MASTER_COUNT        = 3
)

// addOwnerRefToObject appends the desired OwnerReference to the object
func addOwnerRefToObject(o metav1.Object, r metav1.OwnerReference) {
	if (metav1.OwnerReference{}) != r {
		o.SetOwnerReferences(append(o.GetOwnerReferences(), r))
	}
}

func isOwner(subject metav1.ObjectMeta, ownerMeta metav1.ObjectMeta) bool {
	for _, ref := range subject.GetOwnerReferences() {
		if ref.UID == ownerMeta.UID {
			return true
		}
	}
	return false
}

func selectorForES(nodeRole string, clusterName string) map[string]string {

	return map[string]string{
		nodeRole:       "true",
		"cluster-name": clusterName,
	}
}

func labelsForESCluster(clusterName string) map[string]string {

	return map[string]string{
		"cluster-name": clusterName,
	}
}

func appendDefaultLabel(clusterName string, labels map[string]string) map[string]string {
	if _, ok := labels["cluster-name"]; ok {
		return labels
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels["cluster-name"] = clusterName
	return labels
}

// asOwner returns an owner reference set as the vault cluster CR
func asOwner(v *api.Elasticsearch) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: api.SchemeGroupVersion.String(),
		Kind:       v.Kind,
		Name:       v.Name,
		UID:        v.UID,
		Controller: &trueVar,
	}
}

func getReadinessProbe() v1.Probe {
	return v1.Probe{
		TimeoutSeconds:      30,
		InitialDelaySeconds: 10,
		FailureThreshold:    15,
		Handler: v1.Handler{
			TCPSocket: &v1.TCPSocketAction{
				Port: intstr.FromInt(9300),
			},
		},
	}
}

func getResourceRequirements(commonResRequirements, nodeResRequirements v1.ResourceRequirements) v1.ResourceRequirements {
	limitCPU := nodeResRequirements.Limits.Cpu()
	if limitCPU.IsZero() {
		if commonResRequirements.Limits.Cpu().IsZero() {
			CPU, _ := resource.ParseQuantity(defaultCPULimit)
			limitCPU = &CPU
		} else {
			limitCPU = commonResRequirements.Limits.Cpu()
		}
	}
	limitMem := nodeResRequirements.Limits.Memory()
	if limitMem.IsZero() {
		if commonResRequirements.Limits.Memory().IsZero() {
			Mem, _ := resource.ParseQuantity(defaultMemoryLimit)
			limitMem = &Mem
		} else {
			limitMem = commonResRequirements.Limits.Memory()
		}

	}
	requestCPU := nodeResRequirements.Requests.Cpu()
	if requestCPU.IsZero() {
		if commonResRequirements.Requests.Cpu().IsZero() {
			CPU, _ := resource.ParseQuantity(defaultCPURequest)
			requestCPU = &CPU
		} else {
			requestCPU = commonResRequirements.Requests.Cpu()
		}
	}
	requestMem := nodeResRequirements.Requests.Memory()
	if requestMem.IsZero() {
		if commonResRequirements.Requests.Memory().IsZero() {
			Mem, _ := resource.ParseQuantity(defaultMemoryRequest)
			requestMem = &Mem
		} else {
			requestMem = commonResRequirements.Requests.Memory()
		}
	}

	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			"cpu":    *limitCPU,
			"memory": *limitMem,
		},
		Requests: v1.ResourceList{
			"cpu":    *requestCPU,
			"memory": *requestMem,
		},
	}

}

func listDeployments(clusterName, namespace string) (*apps.DeploymentList, error) {
	list := deploymentList()
	labelSelector := labels.SelectorFromSet(labelsForESCluster(clusterName)).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err := sdk.List(namespace, list, sdk.WithListOptions(listOps))
	if err != nil {
		return list, fmt.Errorf("Unable to list deployments: %v", err)
	}

	return list, nil
}

func listReplicaSets(clusterName, namespace string) (*apps.ReplicaSetList, error) {
	list := replicaSetList()
	labelSelector := labels.SelectorFromSet(labelsForESCluster(clusterName)).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err := sdk.List(namespace, list, sdk.WithListOptions(listOps))
	if err != nil {
		return list, fmt.Errorf("Unable to list ReplicaSets: %v", err)
	}

	return list, nil
}

func listStatefulSets(clusterName, namespace string) (*apps.StatefulSetList, error) {
	list := statefulSetList()
	labelSelector := labels.SelectorFromSet(labelsForESCluster(clusterName)).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err := sdk.List(namespace, list, sdk.WithListOptions(listOps))
	if err != nil {
		return list, fmt.Errorf("Unable to list StatefulSets: %v", err)
	}

	return list, nil
}

func statefulSetList() *apps.StatefulSetList {
	return &apps.StatefulSetList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
	}
}

func deploymentList() *apps.DeploymentList {
	return &apps.DeploymentList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
	}
}

func popDeployment(deployments *apps.DeploymentList, cfg desiredNodeState) (*apps.DeploymentList, apps.Deployment, bool) {
	var deployment apps.Deployment
	var index = -1
	for i, dpl := range deployments.Items {
		if dpl.Name == cfg.DeployName {
			deployment = dpl
			index = i
			break
		}
	}
	if index == -1 {
		return deployments, deployment, false
	}
	dpls := deploymentList()
	deployments.Items[index] = deployments.Items[len(deployments.Items)-1]
	dpls.Items = deployments.Items[:len(deployments.Items)-1]
	return dpls, deployment, true
}

func replicaSetList() *apps.ReplicaSetList {
	return &apps.ReplicaSetList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "apps/v1",
		},
	}
}

func popReplicaSet(replicaSets *apps.ReplicaSetList, cfg actualNodeState) (*apps.ReplicaSetList, apps.ReplicaSet, bool) {
	var replicaSet apps.ReplicaSet
	var index = -1
	if cfg.Deployment == nil {
		return replicaSets, replicaSet, false
	}
	for i, rsItem := range replicaSets.Items {
		if isOwner(rsItem.ObjectMeta, cfg.Deployment.ObjectMeta) {
			replicaSet = rsItem
			index = i
			break
		}
	}
	if index == -1 {
		return replicaSets, replicaSet, false
	}
	rsList := replicaSetList()
	replicaSets.Items[index] = replicaSets.Items[len(replicaSets.Items)-1]
	rsList.Items = replicaSets.Items[:len(replicaSets.Items)-1]
	return rsList, replicaSet, true
}

func popPod(pods *v1.PodList, cfg actualNodeState) (*v1.PodList, v1.Pod, bool) {
	var (
		pod              v1.Pod
		index            = -1
		parentObjectMeta metav1.ObjectMeta
	)
	if cfg.ReplicaSet != nil {
		parentObjectMeta = cfg.ReplicaSet.ObjectMeta
	} else if cfg.StatefulSet != nil {
		parentObjectMeta = cfg.StatefulSet.ObjectMeta
	} else {
		return pods, pod, false
	}
	for i, podItem := range pods.Items {
		if isOwner(podItem.ObjectMeta, parentObjectMeta) {
			pod = podItem
			index = i
			break
		}
	}
	if index == -1 {
		return pods, pod, false
	}
	podList := podList()
	pods.Items[index] = pods.Items[len(pods.Items)-1]
	podList.Items = pods.Items[:len(pods.Items)-1]
	return podList, pod, true

}

// podList returns a v1.PodList object
func podList() *v1.PodList {
	return &v1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
	}
}

func listPods(clusterName, namespace string) (*v1.PodList, error) {
	podList := podList()
	labelSelector := labels.SelectorFromSet(labelsForESCluster(clusterName)).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err := sdk.List(namespace, podList, sdk.WithListOptions(listOps))
	if err != nil {
		return podList, fmt.Errorf("failed to list pods: %v", err)
	}
	return podList, nil
}

func listRunningPods(clusterName, namespace string) (*v1.PodList, error) {
	pods, err := listPods(clusterName, namespace)
	if err != nil {
		return nil, err
	}
	// empty slice with memory allocated for len(pods.Items) v1.Pod objects
	runningPods := make([]v1.Pod, 0, len(pods.Items))
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			runningPods = append(runningPods, pod)
		}
	}
	result := podList()
	result.Items = runningPods
	return result, nil
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []v1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

func popStatefulSet(statefulSets *apps.StatefulSetList, cfg desiredNodeState) (*apps.StatefulSetList, apps.StatefulSet, bool) {
	var statefulSet apps.StatefulSet
	var index = -1
	for i, ss := range statefulSets.Items {
		if ss.Name == cfg.DeployName {
			statefulSet = ss
			index = i
			break
		}
	}
	if index == -1 {
		return statefulSets, statefulSet, false
	}
	dpls := statefulSetList()
	statefulSets.Items[index] = statefulSets.Items[len(statefulSets.Items)-1]
	dpls.Items = statefulSets.Items[:len(statefulSets.Items)-1]
	return dpls, statefulSet, true
}

func getMasterCount(dpl *v1alpha1.Elasticsearch) int32 {
	masterCount := int32(0)

	for _, node := range dpl.Spec.Nodes {
		if isNodeMaster(node) {
			masterCount = masterCount + node.Replicas
		}
	}

	return masterCount
}

func getDataCount(dpl *v1alpha1.Elasticsearch) int32 {
	dataCount := int32(0)

	for _, node := range dpl.Spec.Nodes {
		if isNodeData(node) {
			dataCount = dataCount + node.Replicas
		}
	}

	return dataCount
}

func isNodeMaster(node v1alpha1.ElasticsearchNode) bool {
	for _, role := range node.Roles {
		if role == v1alpha1.ElasticsearchRoleMaster {
			return true
		}
	}
	return false
}

func isNodeData(node v1alpha1.ElasticsearchNode) bool {
	for _, role := range node.Roles {
		if role == v1alpha1.ElasticsearchRoleData {
			return true
		}
	}
	return false
}

func isNodeClient(node *v1alpha1.ElasticsearchNode) bool {
	for _, role := range node.Roles {
		if role == v1alpha1.ElasticsearchRoleClient {
			return true
		}
	}
	return false
}

func isValidMasterCount(cluster *v1alpha1.Elasticsearch) bool {
	masterCount := int32(0)

	for _, node := range cluster.Spec.Nodes {
		if isNodeMaster(node) {
			masterCount = masterCount + node.Replicas
		}
	}

	return (masterCount <= MAX_MASTER_COUNT)
}

func lookupEnvWithDefault(envName, defaultValue string) string {
	if value, ok := os.LookupEnv(envName); ok {
		return value
	}
	return defaultValue
}
