package utils

import (
	"fmt"

	"github.com/pkg/errors"
	rv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	rv1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	v1beta2 "k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/intstr"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
)

const (
	running = "Running"
)

func CreateOrUpdateEmbeddedTarget(dep rv1beta2.DeploymentInterface, sa rv1.ServiceAccountInterface, se rv1.ServiceInterface, ro rrbacv1.RoleInterface, rb rrbacv1.RoleBindingInterface, namespace string, obj *v3.ClusterLogging) error {
	// create es deployment
	_, err := dep.Controller().Lister().Get(loggingconfig.LoggingNamespace, loggingconfig.EmbeddedESName)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "get deployment %s fail", loggingconfig.EmbeddedESName)
	}

	// create service account, role and rolebinding
	sc := newESServiceAccount(namespace)
	role := newESRole(namespace)
	roleBind := newESRoleBinding(namespace)

	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = sa.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("recycle %s service account failed", loggingconfig.EmbeddedESName)
			}
		}
	}()
	_, err = sa.Create(sc)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "create service account %s fail", loggingconfig.EmbeddedESName)
	}

	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = ro.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("recycle %s role failed", loggingconfig.EmbeddedESName)
			}
		}
	}()
	_, err = ro.Create(role)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "create role %s fail", loggingconfig.EmbeddedESName)
	}

	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = rb.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("recycle %s role binding failed", loggingconfig.EmbeddedESName)
			}
		}
	}()
	_, err = rb.Create(roleBind)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "create role %s fail", loggingconfig.EmbeddedESName)
	}

	// create service and deployment
	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = se.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("recycle %s service failed", loggingconfig.EmbeddedESName)
			}
		}
	}()
	newService := newESService(namespace)
	_, err = se.Create(newService)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "create service %s fail", loggingconfig.EmbeddedESName)
	}

	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = dep.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("recycle %s deployment failed", loggingconfig.EmbeddedESName)
			}
		}
	}()
	esDeployment := newESDeployment(namespace, obj)
	_, err = dep.Create(esDeployment)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "create deployment %s fail", loggingconfig.EmbeddedESName)
	}

	if err = updateEmbeddedQuota(dep, obj); err != nil {
		return err
	}

	// create kibana deployment
	_, err = dep.Controller().Lister().Get(loggingconfig.LoggingNamespace, loggingconfig.EmbeddedKibanaName)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "get deployment %s fail", loggingconfig.EmbeddedKibanaName)
	}

	// create service account, role and rolebinding
	sc = newKibanaServiceAccount(namespace)
	role = newKibanaRole(namespace)
	roleBind = newKibanaRoleBinding(namespace)

	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = sa.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("recycle %s service account failed", loggingconfig.EmbeddedKibanaName)
			}

		}
	}()
	_, err = sa.Create(sc)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "create service account  %s fail", loggingconfig.EmbeddedKibanaName)
	}

	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = ro.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("recycle %s role failed", loggingconfig.EmbeddedKibanaName)
			}
		}
	}()
	_, err = ro.Create(role)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "create role %s fail", loggingconfig.EmbeddedKibanaName)
	}

	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = rb.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("recycle %s role binding failed", loggingconfig.EmbeddedKibanaName)
			}
		}
	}()
	_, err = rb.Create(roleBind)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "create role %s fail", loggingconfig.EmbeddedKibanaName)
	}

	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = se.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("recycle %s service failed", loggingconfig.EmbeddedKibanaName)
			}
		}
	}()
	newService = newKibanaService(namespace)
	_, err = se.Create(newService)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "create service %s fail", loggingconfig.EmbeddedKibanaName)
	}

	defer func() {
		if err != nil && !apierrors.IsAlreadyExists(err) {
			if err = dep.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("recycle %s deployment failed", loggingconfig.EmbeddedKibanaName)
			}
		}
	}()
	kibanaDeployment := newKibanaDeployment(namespace)
	_, err = dep.Create(kibanaDeployment)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "create deployment %s fail", loggingconfig.EmbeddedKibanaName)
	}
	return nil
}

func RemoveEmbeddedTarget(dep rv1beta2.DeploymentInterface, sa rv1.ServiceAccountInterface, se rv1.ServiceInterface, ro rrbacv1.RoleInterface, rb rrbacv1.RoleBindingInterface) error {
	//service account
	var errgrp errgroup.Group
	errgrp.Go(func() error {
		return se.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
	})
	errgrp.Go(func() error {
		return sa.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
	})

	//role
	errgrp.Go(func() error {
		return ro.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
	})
	errgrp.Go(func() error {
		return ro.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
	})

	//rolebinding
	errgrp.Go(func() error {
		return rb.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
	})
	errgrp.Go(func() error {
		return rb.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
	})

	//service
	errgrp.Go(func() error {
		return se.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{})
	})
	errgrp.Go(func() error {
		return se.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{})
	})

	//deployment
	deleteOp := metav1.DeletePropagationBackground
	errgrp.Go(func() error {
		return dep.Delete(loggingconfig.EmbeddedESName, &metav1.DeleteOptions{PropagationPolicy: &deleteOp})
	})
	errgrp.Go(func() error {
		return dep.Delete(loggingconfig.EmbeddedKibanaName, &metav1.DeleteOptions{PropagationPolicy: &deleteOp})
	})
	if err := errgrp.Wait(); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func UpdateEmbeddedEndpoint(podLister rv1.PodLister, serviceLister rv1.ServiceLister, clusterLoggings v3.ClusterLoggingInterface) error {
	cls, err := clusterLoggings.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("get cluterlogging failed, %v", err)
	}
	if len(cls.Items) == 0 {
		return fmt.Errorf("not clusterlogging found")
	}
	cl := cls.Items[0]
	if cl.Spec.EmbeddedConfig == nil {
		return fmt.Errorf("embedded configuration should not be nil when update embedded endpoint")
	}

	updated := false
	esEndpoint, err := getEmbeddedESEndpoint(podLister, serviceLister)
	if err != nil {
		return fmt.Errorf("get elasticsearch endpoint failed, %v", err)
	}

	if cl.Spec.EmbeddedConfig.ElasticsearchEndpoint != esEndpoint {
		updated = true
		cl.Spec.EmbeddedConfig.ElasticsearchEndpoint = esEndpoint
	}

	kibanaEndpoint, err := getEmbeddedKibanaEndpoint(podLister, serviceLister)
	if err != nil {
		return fmt.Errorf("get kibana endpoint failed, %v", err)
	}

	if cl.Spec.EmbeddedConfig.KibanaEndpoint != kibanaEndpoint {
		updated = true
		cl.Spec.EmbeddedConfig.KibanaEndpoint = kibanaEndpoint
	}

	if !updated {
		return nil
	}

	if _, err = clusterLoggings.Update(&cl); err != nil {
		return fmt.Errorf("update embedded logging endpoint failed, %v", err)
	}

	if esEndpoint == "" || kibanaEndpoint == "" {
		return fmt.Errorf("embedded endpoint not set completely")
	}
	return nil
}

func updateEmbeddedQuota(dep rv1beta2.DeploymentInterface, obj *v3.ClusterLogging) error {
	d, err := dep.Get(loggingconfig.EmbeddedESName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "fail to get embedded deployment %s before update quota", loggingconfig.EmbeddedESName)
	}
	requests, limits := map[v1.ResourceName]resource.Quantity{}, map[v1.ResourceName]resource.Quantity{}
	if obj.Spec.EmbeddedConfig.LimitsCPU > 0 {
		limits[v1.ResourceCPU] = *resource.NewMilliQuantity(int64(obj.Spec.EmbeddedConfig.LimitsCPU), resource.DecimalSI)
	}
	if obj.Spec.EmbeddedConfig.LimitsMemery > 0 {
		limits[v1.ResourceMemory] = *resource.NewQuantity(int64(obj.Spec.EmbeddedConfig.LimitsMemery*1024*1024), resource.DecimalSI)
	}

	if obj.Spec.EmbeddedConfig.RequestsCPU > 0 {
		requests[v1.ResourceCPU] = *resource.NewMilliQuantity(int64(obj.Spec.EmbeddedConfig.RequestsCPU), resource.DecimalSI)
	}
	if obj.Spec.EmbeddedConfig.RequestsMemery > 0 {
		requests[v1.ResourceMemory] = *resource.NewQuantity(int64(obj.Spec.EmbeddedConfig.RequestsMemery*1024*1024), resource.DecimalSI)
	}

	d.Spec.Template.Spec.Containers[0].Resources.Requests = requests
	d.Spec.Template.Spec.Containers[0].Resources.Limits = limits
	_, err = dep.Update(d)
	if err != nil {
		return errors.Wrapf(err, "update deployment %s fail", loggingconfig.EmbeddedESName)
	}
	return nil
}

func getEmbeddedESEndpoint(podLister rv1.PodLister, serviceLister rv1.ServiceLister) (esEndpoint string, err error) {
	selector := labels.NewSelector()
	requirement, err := labels.NewRequirement(loggingconfig.LabelK8sApp, selection.Equals, []string{loggingconfig.EmbeddedESName})
	if err != nil {
		return "", err
	}

	espods, err := podLister.List(loggingconfig.LoggingNamespace, selector.Add(*requirement))
	if err != nil {
		return "", err
	}
	esservice, err := serviceLister.Get(loggingconfig.LoggingNamespace, loggingconfig.EmbeddedESName)
	if err != nil {
		return "", err
	}

	if len(esservice.Spec.Ports) == 0 {
		return "", fmt.Errorf("get service %s node port failed", loggingconfig.EmbeddedESName)
	}
	var esPort int32
	for _, v := range esservice.Spec.Ports {
		if v.Name == "http" {
			esPort = v.NodePort
			break
		}
	}

	if len(espods) == 0 {
		return "", fmt.Errorf("deploying %s", loggingconfig.EmbeddedESName)
	}
	espod := espods[0]
	if espod.Status.Phase == running {
		return fmt.Sprintf("http://%s:%v", espod.Status.HostIP, esPort), nil
	}
	return "", fmt.Errorf("got embedded elasticsearch pod status %s", espod.Status.Phase)
}

func getEmbeddedKibanaEndpoint(podLister rv1.PodLister, serviceLister rv1.ServiceLister) (kibanaEndpoint string, err error) {
	selector := labels.NewSelector()
	requirement, err := labels.NewRequirement(loggingconfig.LabelK8sApp, selection.Equals, []string{loggingconfig.EmbeddedKibanaName})
	if err != nil {
		return "", err
	}
	kibanapods, err := podLister.List(loggingconfig.LoggingNamespace, selector.Add(*requirement))
	if err != nil {
		return "", err
	}
	kibanaservice, err := serviceLister.Get(loggingconfig.LoggingNamespace, loggingconfig.EmbeddedKibanaName)
	if err != nil {
		return "", err
	}
	if len(kibanaservice.Spec.Ports) == 0 {
		return "", fmt.Errorf("get service %s node port failed", loggingconfig.EmbeddedKibanaName)
	}

	if len(kibanapods) == 0 {
		return "", fmt.Errorf("deploying %s", loggingconfig.EmbeddedKibanaName)
	}
	kibanapod := kibanapods[0]
	if kibanapod.Status.Phase == running {
		return fmt.Sprintf("http://%s:%v", kibanapod.Status.HostIP, kibanaservice.Spec.Ports[0].NodePort), nil
	}
	return "", fmt.Errorf("got embedded kibana pod status %s", kibanapod.Status.Phase)

}

func newESServiceAccount(namespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedESName,
			Namespace: namespace,
		},
	}
}

func newKibanaServiceAccount(namespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedKibanaName,
			Namespace: namespace,
		},
	}
}

func newESRole(namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedESName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{rbacv1.APIGroupAll},
				Resources: []string{"endpoints"},
				Verbs:     []string{rbacv1.VerbAll},
			},
		},
	}
}

func newKibanaRole(namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedKibanaName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{rbacv1.APIGroupAll},
				Resources: []string{"endpoints"},
				Verbs:     []string{rbacv1.VerbAll},
			},
		},
	}
}

func newESRoleBinding(namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedESName,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			Name:     loggingconfig.EmbeddedESName,
			Kind:     "Role",
			APIGroup: rbacv1.GroupName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      loggingconfig.EmbeddedESName,
				Namespace: namespace,
			},
		},
	}
}

func newKibanaRoleBinding(namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      loggingconfig.EmbeddedKibanaName,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			Name:     loggingconfig.EmbeddedKibanaName,
			Kind:     "Role",
			APIGroup: rbacv1.GroupName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      loggingconfig.EmbeddedKibanaName,
				Namespace: namespace,
			},
		},
	}
}

func newESService(namespace string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      loggingconfig.EmbeddedESName,
			Labels: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.EmbeddedESName,
			},
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				v1.ServicePort{
					Name:       "http",
					Port:       9200,
					TargetPort: intstr.FromInt(9200),
				},
				v1.ServicePort{
					Name:       "tcp",
					Port:       9300,
					TargetPort: intstr.FromInt(9300),
				},
			},
			Selector: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.EmbeddedESName,
			},
		},
	}
}

func newKibanaService(namespace string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      loggingconfig.EmbeddedKibanaName,
			Labels: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.EmbeddedKibanaName,
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				v1.ServicePort{
					Name:       "http",
					Port:       5601,
					TargetPort: intstr.FromInt(5601),
				},
			},
			Type: v1.ServiceTypeNodePort,
			Selector: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.EmbeddedKibanaName,
			},
		},
	}
}

func newESDeployment(namespace string, obj *v3.ClusterLogging) *v1beta2.Deployment {
	limits := map[v1.ResourceName]resource.Quantity{}
	if obj.Spec.EmbeddedConfig.LimitsCPU > 0 {
		//CPU is always requested as an absolute quantity, never as a relative quantity; 0.1 is the same amount of CPU on a single-core, dual-core, or 48-core machine
		limits[v1.ResourceCPU] = *resource.NewMilliQuantity(int64(obj.Spec.EmbeddedConfig.LimitsCPU), resource.DecimalSI)
	}
	if obj.Spec.EmbeddedConfig.LimitsMemery > 0 {
		//Limits and requests for memory are measured in bytes.
		limits[v1.ResourceMemory] = *resource.NewQuantity(int64(obj.Spec.EmbeddedConfig.LimitsMemery*1024*1024), resource.DecimalSI)
	}

	deployment := &v1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      loggingconfig.EmbeddedESName,
			Labels: map[string]string{
				loggingconfig.LabelK8sApp: loggingconfig.EmbeddedESName,
			},
		},
		Spec: v1beta2.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					loggingconfig.LabelK8sApp: loggingconfig.EmbeddedESName,
				},
			},
			Replicas: int32Ptr(1),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Labels: map[string]string{
						loggingconfig.LabelK8sApp: loggingconfig.EmbeddedESName,
					},
				},
				Spec: v1.PodSpec{
					ServiceAccountName: loggingconfig.EmbeddedESName,
					InitContainers: []v1.Container{
						{
							Name:            "init-sysctl",
							Image:           v3.ToolsSystemImages.LoggingSystemImages.Busybox,
							ImagePullPolicy: v1.PullIfNotPresent,
							Command:         []string{"sysctl", "-w", "vm.max_map_count=262144"},
							SecurityContext: &v1.SecurityContext{
								Privileged: boolPtr(true),
							},
						},
					},
					Containers: []v1.Container{
						{
							Name: loggingconfig.EmbeddedESName,
							SecurityContext: &v1.SecurityContext{
								Capabilities: &v1.Capabilities{
									Add: []v1.Capability{"IPC_LOCK"},
								},
							},
							Image: v3.ToolsSystemImages.LoggingSystemImages.Elaticsearch,
							Env: []v1.EnvVar{
								{
									Name:  "KUBERNETES_CA_CERTIFICATE_FILE",
									Value: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
								},
								{
									Name: "NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name:  "CLUSTER_NAME",
									Value: "myesdb",
								},
								{
									Name:  "DISCOVERY_SERVICE",
									Value: loggingconfig.EmbeddedESName,
								},
								{
									Name:  "NODE_MASTER",
									Value: "true",
								},
								{
									Name:  "NODE_DATA",
									Value: "true",
								},
								{
									Name:  "HTTP_ENABLE",
									Value: "true",
								},
							},
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									Protocol:      v1.ProtocolTCP,
									ContainerPort: 9200,
								},
								{
									Name:          "tcp",
									Protocol:      v1.ProtocolTCP,
									ContainerPort: 9300,
								},
							},
							Resources: v1.ResourceRequirements{
								Requests: map[v1.ResourceName]resource.Quantity{
									//CPU is always requested as an absolute quantity, never as a relative quantity; 0.1 is the same amount of CPU on a single-core, dual-core, or 48-core machine
									v1.ResourceCPU: *resource.NewMilliQuantity(int64(obj.Spec.EmbeddedConfig.RequestsCPU), resource.DecimalSI),
									//Limits and requests for memory are measured in bytes.
									v1.ResourceMemory: *resource.NewQuantity(int64(obj.Spec.EmbeddedConfig.RequestsMemery*1024*1024), resource.DecimalSI), // unit is byte
								},
								Limits: limits,
							},
							VolumeMounts: []v1.VolumeMount{
								{
									MountPath: "/data",
									Name:      "storage",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "storage",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyAlways,
				},
			},
		},
	}
	return deployment
}

func newKibanaDeployment(namespace string) *v1beta2.Deployment {
	deployment := &v1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      loggingconfig.EmbeddedKibanaName,
		},
		Spec: v1beta2.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					loggingconfig.LabelK8sApp: loggingconfig.EmbeddedKibanaName,
				},
			},
			Replicas: int32Ptr(1),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Labels: map[string]string{
						loggingconfig.LabelK8sApp: loggingconfig.EmbeddedKibanaName,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  loggingconfig.EmbeddedKibanaName,
							Image: v3.ToolsSystemImages.LoggingSystemImages.Kibana,
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									Protocol:      v1.ProtocolTCP,
									ContainerPort: 5601,
								},
							},
							Env: []v1.EnvVar{
								{
									Name:  "ELASTICSEARCH_URL",
									Value: "http://" + loggingconfig.EmbeddedESName + "." + namespace + ":9200",
								},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyAlways,
				},
			},
		},
	}

	return deployment
}

func int32Ptr(i int32) *int32 { return &i }

func boolPtr(b bool) *bool { return &b }
