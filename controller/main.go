package main

import (
	"context"
	"fmt"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TracingConfig represents our custom resource
type TracingConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TracingConfigSpec   `json:"spec,omitempty"`
	Status            TracingConfigStatus `json:"status,omitempty"`
}

type TracingConfigSpec struct {
	Enabled       bool                  `json:"enabled"`
	SamplingRate  float64               `json:"samplingRate,omitempty"`
	Endpoint      string                `json:"endpoint"`
	ServiceName   string                `json:"serviceName"`
	Namespace     string                `json:"namespace,omitempty"`
	Selector      *metav1.LabelSelector `json:"selector,omitempty"`
	Headers       map[string]string     `json:"headers,omitempty"`
	Attributes    map[string]string     `json:"attributes,omitempty"`
	ExportTimeout string                `json:"exportTimeout,omitempty"`
	BatchTimeout  string                `json:"batchTimeout,omitempty"`
	MaxBatchSize  int                   `json:"maxBatchSize,omitempty"`
}

type TracingConfigStatus struct {
	Phase      string       `json:"phase,omitempty"`
	Message    string       `json:"message,omitempty"`
	AppliedAt  *metav1.Time `json:"appliedAt,omitempty"`
	TargetPods []string     `json:"targetPods,omitempty"`
}

type TracingConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TracingConfig `json:"items"`
}

// DeepCopyObject implements runtime.Object interface
func (tc *TracingConfig) DeepCopyObject() runtime.Object {
	if tc == nil {
		return nil
	}
	out := new(TracingConfig)
	tc.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (tc *TracingConfig) DeepCopyInto(out *TracingConfig) {
	*out = *tc
	out.TypeMeta = tc.TypeMeta
	tc.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	tc.Spec.DeepCopyInto(&out.Spec)
	tc.Status.DeepCopyInto(&out.Status)
}

// DeepCopy creates a deep copy of the TracingConfig
func (tc *TracingConfig) DeepCopy() *TracingConfig {
	if tc == nil {
		return nil
	}
	out := new(TracingConfig)
	tc.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (tcs *TracingConfigSpec) DeepCopyInto(out *TracingConfigSpec) {
	*out = *tcs
	if tcs.Selector != nil {
		in, out := &tcs.Selector, &out.Selector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if tcs.Headers != nil {
		in, out := &tcs.Headers, &out.Headers
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if tcs.Attributes != nil {
		in, out := &tcs.Attributes, &out.Attributes
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (tcs *TracingConfigStatus) DeepCopyInto(out *TracingConfigStatus) {
	*out = *tcs
	if tcs.AppliedAt != nil {
		in, out := &tcs.AppliedAt, &out.AppliedAt
		*out = (*in).DeepCopy()
	}
	if tcs.TargetPods != nil {
		in, out := &tcs.TargetPods, &out.TargetPods
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopyObject implements runtime.Object interface
func (tcl *TracingConfigList) DeepCopyObject() runtime.Object {
	if tcl == nil {
		return nil
	}
	out := new(TracingConfigList)
	tcl.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (tcl *TracingConfigList) DeepCopyInto(out *TracingConfigList) {
	*out = *tcl
	out.TypeMeta = tcl.TypeMeta
	tcl.ListMeta.DeepCopyInto(&out.ListMeta)
	if tcl.Items != nil {
		in, out := &tcl.Items, &out.Items
		*out = make([]TracingConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// TracingConfigReconciler reconciles TracingConfig objects
type TracingConfigReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	K8sClient kubernetes.Interface
}

func (r *TracingConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Printf("Reconciling TracingConfig %s/%s", req.Namespace, req.Name)

	// Fetch the TracingConfig instance
	var tracingConfig TracingConfig
	if err := r.Get(ctx, req.NamespacedName, &tracingConfig); err != nil {
		log.Printf("Unable to fetch TracingConfig: %v", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Update status to Pending
	tracingConfig.Status.Phase = "Pending"
	tracingConfig.Status.Message = "Processing tracing configuration"
	if err := r.Status().Update(ctx, &tracingConfig); err != nil {
		log.Printf("Failed to update status to Pending: %v", err)
	}

	// Find target pods based on selector
	targetNamespace := tracingConfig.Spec.Namespace
	if targetNamespace == "" {
		targetNamespace = tracingConfig.Namespace
	}

	var pods corev1.PodList
	listOpts := []client.ListOption{
		client.InNamespace(targetNamespace),
	}

	if tracingConfig.Spec.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(tracingConfig.Spec.Selector)
		if err != nil {
			log.Printf("Invalid label selector: %v", err)
			tracingConfig.Status.Phase = "Failed"
			tracingConfig.Status.Message = fmt.Sprintf("Invalid label selector: %v", err)
			r.Status().Update(ctx, &tracingConfig)
			return ctrl.Result{}, err
		}
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: selector})
	}

	if err := r.List(ctx, &pods, listOpts...); err != nil {
		log.Printf("Failed to list pods: %v", err)
		tracingConfig.Status.Phase = "Failed"
		tracingConfig.Status.Message = fmt.Sprintf("Failed to list pods: %v", err)
		r.Status().Update(ctx, &tracingConfig)
		return ctrl.Result{}, err
	}

	// Create or update ConfigMap with tracing configuration
	configMapName := fmt.Sprintf("%s-tracing-config", tracingConfig.Name)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: targetNamespace,
		},
		Data: map[string]string{
			"OTEL_EXPORTER_OTLP_ENDPOINT": tracingConfig.Spec.Endpoint,
			"OTEL_SERVICE_NAME":           tracingConfig.Spec.ServiceName,
			"OTEL_TRACES_SAMPLER":         "traceidratio",
			"OTEL_TRACES_SAMPLER_ARG":     fmt.Sprintf("%.2f", tracingConfig.Spec.SamplingRate),
		},
	}

	// Add optional configurations
	if tracingConfig.Spec.ExportTimeout != "" {
		configMap.Data["OTEL_EXPORTER_OTLP_TIMEOUT"] = tracingConfig.Spec.ExportTimeout
	}
	if tracingConfig.Spec.BatchTimeout != "" {
		configMap.Data["OTEL_BSP_SCHEDULE_DELAY"] = tracingConfig.Spec.BatchTimeout
	}
	if tracingConfig.Spec.MaxBatchSize > 0 {
		configMap.Data["OTEL_BSP_MAX_EXPORT_BATCH_SIZE"] = fmt.Sprintf("%d", tracingConfig.Spec.MaxBatchSize)
	}

	// Add custom attributes
	for key, value := range tracingConfig.Spec.Attributes {
		configMap.Data[fmt.Sprintf("OTEL_RESOURCE_ATTRIBUTES_%s", key)] = value
	}

	// Create or update the ConfigMap
	existingConfigMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: targetNamespace}, existingConfigMap)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			// ConfigMap doesn't exist, create it
			if err := r.Create(ctx, configMap); err != nil {
				log.Printf("Failed to create ConfigMap: %v", err)
				tracingConfig.Status.Phase = "Failed"
				tracingConfig.Status.Message = fmt.Sprintf("Failed to create ConfigMap: %v", err)
				r.Status().Update(ctx, &tracingConfig)
				return ctrl.Result{}, err
			}
			log.Printf("Created ConfigMap %s/%s", targetNamespace, configMapName)
		} else {
			log.Printf("Failed to get ConfigMap: %v", err)
			return ctrl.Result{}, err
		}
	} else {
		// ConfigMap exists, update it
		existingConfigMap.Data = configMap.Data
		if err := r.Update(ctx, existingConfigMap); err != nil {
			log.Printf("Failed to update ConfigMap: %v", err)
			tracingConfig.Status.Phase = "Failed"
			tracingConfig.Status.Message = fmt.Sprintf("Failed to update ConfigMap: %v", err)
			r.Status().Update(ctx, &tracingConfig)
			return ctrl.Result{}, err
		}
		log.Printf("Updated ConfigMap %s/%s", targetNamespace, configMapName)
	}

	// Update deployments to use the tracing configuration
	var deployments appsv1.DeploymentList
	if err := r.List(ctx, &deployments, listOpts...); err != nil {
		log.Printf("Failed to list deployments: %v", err)
	} else {
		for _, deployment := range deployments.Items {
			updated := false
			for i := range deployment.Spec.Template.Spec.Containers {
				container := &deployment.Spec.Template.Spec.Containers[i]

				// Add environment variables from ConfigMap
				envFromExists := false
				for _, envFrom := range container.EnvFrom {
					if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == configMapName {
						envFromExists = true
						break
					}
				}

				if !envFromExists {
					container.EnvFrom = append(container.EnvFrom, corev1.EnvFromSource{
						ConfigMapRef: &corev1.ConfigMapEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMapName,
							},
						},
					})
					updated = true
				}
			}

			if updated {
				if err := r.Update(ctx, &deployment); err != nil {
					log.Printf("Failed to update deployment %s: %v", deployment.Name, err)
				} else {
					log.Printf("Updated deployment %s with tracing configuration", deployment.Name)
				}
			}
		}
	}

	// Collect target pod names
	var targetPodNames []string
	for _, pod := range pods.Items {
		targetPodNames = append(targetPodNames, pod.Name)
	}

	// Update status to Applied
	now := metav1.Now()
	tracingConfig.Status.Phase = "Applied"
	tracingConfig.Status.Message = fmt.Sprintf("Tracing configuration applied to %d pods", len(targetPodNames))
	tracingConfig.Status.AppliedAt = &now
	tracingConfig.Status.TargetPods = targetPodNames

	if err := r.Status().Update(ctx, &tracingConfig); err != nil {
		log.Printf("Failed to update status: %v", err)
		return ctrl.Result{}, err
	}

	log.Printf("Successfully reconciled TracingConfig %s/%s", req.Namespace, req.Name)
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

func (r *TracingConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&TracingConfig{}).
		Complete(r)
}

func main() {
	log.Println("Starting Tracing Controller")

	// Get Kubernetes config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in-cluster config: %v", err)
	}

	// Create Kubernetes client
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Setup scheme
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		log.Fatalf("Failed to add core/v1 to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		log.Fatalf("Failed to add apps/v1 to scheme: %v", err)
	}

	// Add our custom resource to the scheme
	gv := schema.GroupVersion{Group: "observability.kubevishwa.io", Version: "v1"}
	scheme.AddKnownTypes(gv, &TracingConfig{}, &TracingConfigList{})
	metav1.AddToGroupVersion(scheme, gv)

	// Create manager
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	// Setup reconciler
	reconciler := &TracingConfigReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		K8sClient: k8sClient,
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		log.Fatalf("Failed to setup controller: %v", err)
	}

	log.Println("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Fatalf("Failed to start manager: %v", err)
	}
}
