/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	azcoreruntime "github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/alertsmanagement/armalertsmanagement"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusmodel "github.com/prometheus/common/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TODO: review the permissions needed.
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules/finalizers,verbs=update

const (
	finalizerName = "promrule-to-azpromrulegroup.digdir.no/finalizer"
	// This annotation has a comma separated string with the names of the resources created in azure.
	azPrometheusRuleGroupResourceNamesAnnotation = "promrule-to-azpromrulegroup.digdir.no/azpromrulegroup-names"
	// This annotation has the latest applied ARM template.
	azArmTemplateHashAnnotation = "promrule-to-azpromrulegroup.digdir.no/latest-arm-template-deployed-hash"
	// This annotation has the latest ARM template deployment name
	azArmDeploymentNameAnnotation = "promrule-to-azpromrulegroup.digdir.no/az-arm-deployment-name"
	// Last time a sucessul deployment was done
	azArmDeploymentLastSuccessfulTimestampAnnotation = "promrule-to-azpromrulegroup.digdir.no/az-arm-deployment-last-successful-timestamp"
)

var (
	allAnnotations = [4]string{
		azPrometheusRuleGroupResourceNamesAnnotation,
		azArmTemplateHashAnnotation,
		azArmDeploymentNameAnnotation,
		azArmDeploymentLastSuccessfulTimestampAnnotation,
	}
)

type PromRuleToAzPromRuleGroupReconciler struct {
	client.Client
	Scheme                     *runtime.Scheme
	DeploymentClient           *armresources.DeploymentsClient
	PrometheusRuleGroupsClient *armalertsmanagement.PrometheusRuleGroupsClient
	AzResourceGroupName        string
	AzResourceGroupLocation    string
	AzAzureMonitorWorkspace    string
	AzClusterName              string
	NodePath                   string
	AzPromRulesConverterPath   string
}

func (r *PromRuleToAzPromRuleGroupReconciler) handleCreation(ctx context.Context, req ctrl.Request, promRule monitoringv1.PrometheusRule) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	armTemplateJsonString, err := r.generateArmTemplateFromPromRule(ctx, promRule)
	if err != nil {
		log.Error(err, "failed to convert the PrometheusRule into an ARM template", "namespace", promRule.Namespace, "name", promRule.Name)
		return ctrl.Result{Requeue: false}, err
	}

	ruleGroupNames := generateRuleGroupNamesAnnotationString(promRule)
	suffix := timestamp()
	deploymentName := generateArmDeploymentName(req, suffix)

	err = r.deployArmTemplate(
		ctx,
		deploymentName,
		armTemplateJsonString,
		os.Getenv("AZ_ACTION_GROUP_ID"),
	)
	if err != nil {
		log.Error(err, "failed to deploy arm template", "namespace", promRule.Namespace, "name", promRule.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	// Update the annotations on the CR
	return r.updateAnnotations(ctx, promRule, ruleGroupNames, hashArmTemplate([]byte(armTemplateJsonString)), deploymentName, suffix)
}

func (r *PromRuleToAzPromRuleGroupReconciler) handleUpdate(ctx context.Context, req ctrl.Request, promRule monitoringv1.PrometheusRule) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	annotations := promRule.GetAnnotations()
	lastGeneratedArmtemplateHash := annotations[azArmTemplateHashAnnotation]
	suffix := timestamp()
	armDeploymentName := generateArmDeploymentName(req, suffix)
	regeneratedArmTemplate, err := r.generateArmTemplateFromPromRule(ctx, promRule)
	if err != nil {
		// TODO: Likely not worth it to reschedule
		return ctrl.Result{Requeue: false}, err
	}
	ruleGroupNames := generateRuleGroupNamesAnnotationString(promRule)
	regeneratedArmTemplateHash := hashArmTemplate([]byte(regeneratedArmTemplate))
	if !(regeneratedArmTemplateHash == lastGeneratedArmtemplateHash) {
		annotations := promRule.GetAnnotations()
		promRuleGroupNames := strings.Split(annotations[azPrometheusRuleGroupResourceNamesAnnotation], ",") // old
		var newNames []string
		for _, rgn := range promRule.Spec.Groups {
			newNames = append(newNames, rgn.Name)
		}
		toDelete := removedGroups(promRuleGroupNames, newNames)
		for _, td := range toDelete {
			_, err := r.deletePrometheusRuleGroup(ctx, td)
			if err != nil {
				log.Error(err, "failed to delete PrometheusRuleGroup", "PrometheusRuleGroupName", td)
			}
		}

		err = r.deployArmTemplate(
			ctx,
			armDeploymentName,
			regeneratedArmTemplate,
			os.Getenv("AZ_ACTION_GROUP_ID"),
		)
		if err != nil {
			log.Error(err, "failed to deploy arm template", "namespace", promRule.Namespace, "name", promRule.Name)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
		// Update the annotations on the CR
		return r.updateAnnotations(ctx, promRule, ruleGroupNames, regeneratedArmTemplateHash, armDeploymentName, suffix)
	} else {
		// TODO: Might make sense to double check that the Azure resources havent been deleted/modified outside the controller too.
	}
	return ctrl.Result{}, err
}

func (r *PromRuleToAzPromRuleGroupReconciler) handleDelete(ctx context.Context, promRule monitoringv1.PrometheusRule) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("deletion of PrometheusRule CR detected", "namespace", promRule.Namespace, "name", promRule.Name)
	// The object is scheduled for deletion so we need to delete the equivalent resources in Azure and then remove the finalizer
	if controllerutil.ContainsFinalizer(&promRule, finalizerName) {
		if err := r.deleteExternalResources(ctx, promRule); err != nil {
			// if fail to delete the external dependency here, return with error so that it can be retried.
			log.Info("failed to delete Azure resources", "namespace", promRule.Namespace, "name", promRule.Name)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		log.Info("removing our finalizer", "namespace", promRule.Namespace, "name", promRule.Name)
		ok := controllerutil.RemoveFinalizer(&promRule, finalizerName)
		if ok {
			if err := r.Update(ctx, &promRule); err != nil {
				log.Info("failed to update object", "namespace", promRule.Namespace, "name", promRule.Name)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
		} else {
			log.Info("failed to removed out finalizer from object", "namespace", promRule.Namespace, "name", promRule.Name)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.New("failed to remove finalizer from object")
		}
	}
	return ctrl.Result{}, nil
}

func (r *PromRuleToAzPromRuleGroupReconciler) addOurFinalizer(ctx context.Context, promRule monitoringv1.PrometheusRule) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("updating the PrometheusRule CR with our finalizer", "namespace", promRule.Namespace, "name", promRule.Name)
	ok := controllerutil.AddFinalizer(&promRule, finalizerName)
	if ok {
		if err := r.Update(ctx, &promRule); err != nil {
			log.Error(err, "failed to update the PrometheusRule CR with our finalizer", "namespace", promRule.Namespace, "name", promRule.Name)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		return ctrl.Result{}, nil
	} else {
		log.Info("failed to add our finalzer to the object", "namespace", promRule.Namespace, "name", promRule.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, errors.New("failed to add our finalzer to the object")
	}
}

func (r *PromRuleToAzPromRuleGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Try to get the object to reconcile
	var originalPrometheusRule monitoringv1.PrometheusRule
	if err := r.Get(ctx, req.NamespacedName, &originalPrometheusRule); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch PrometheusRule", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, err
	}

	// The resource is not marked for deletion.
	if originalPrometheusRule.GetDeletionTimestamp().IsZero() {
		// We need to make sure we add a finalizer to the PrometheusRule CR so we can cleanup Azure resources when the CR is deleted.
		if !controllerutil.ContainsFinalizer(&originalPrometheusRule, finalizerName) {
			result, err := r.addOurFinalizer(ctx, originalPrometheusRule)
			return result, err
		}
		// Look into the object's annotations for annotations we own.
		annotations := originalPrometheusRule.GetAnnotations()
		ok := hasAllAnnotations(annotations)
		if !ok {
			log.Info("new PrometheusRule CR detected", "namespace", originalPrometheusRule.Namespace, "name", originalPrometheusRule.Name)
			// A new resource
			result, err := r.handleCreation(ctx, req, originalPrometheusRule)
			return result, err
		} else {
			log.Info("update to PrometheusRule CR detected", "namespace", originalPrometheusRule.Namespace, "name", originalPrometheusRule.Name)
			// Not a new resource, make sure the current state matches the current spec
			result, err := r.handleUpdate(ctx, req, originalPrometheusRule)
			return result, err
		}
	} else {
		result, err := r.handleDelete(ctx, originalPrometheusRule)
		return result, err
	}
}

func (r *PromRuleToAzPromRuleGroupReconciler) updateAnnotations(ctx context.Context, promRule monitoringv1.PrometheusRule, groupNames, regeneratedArmTemplateHash, armDeploymentName, suffix string) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	annotations := promRule.GetAnnotations()
	annotations[azPrometheusRuleGroupResourceNamesAnnotation] = groupNames
	annotations[azArmTemplateHashAnnotation] = regeneratedArmTemplateHash
	annotations[azArmDeploymentNameAnnotation] = armDeploymentName
	annotations[azArmDeploymentLastSuccessfulTimestampAnnotation] = suffix

	promRule.SetAnnotations(annotations)
	err := r.Client.Update(ctx, &promRule)
	if err != nil {
		log.Error(err, "failed to update the PrometheusRule CR with new annotations", "namespace", promRule.Namespace, "name", promRule.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}
	return ctrl.Result{}, nil
}

func (r *PromRuleToAzPromRuleGroupReconciler) deployArmTemplate(ctx context.Context, deploymentName string, jsonTemplate string, actionGroupId string) error {
	log := log.FromContext(ctx)

	contents := make(map[string]interface{})
	_ = json.Unmarshal([]byte(jsonTemplate), &contents)
	deploy, err := r.DeploymentClient.BeginCreateOrUpdate(
		ctx,
		r.AzResourceGroupName,
		deploymentName,
		armresources.Deployment{
			Properties: &armresources.DeploymentProperties{
				Template: contents,
				Mode:     to.Ptr(armresources.DeploymentModeIncremental),
				Parameters: map[string]interface{}{
					"location": map[string]string{
						"value": r.AzResourceGroupLocation,
					},
					"actionGroupId": map[string]string{
						"value": actionGroupId,
					},
					"azureMonitorWorkspace": map[string]string{
						"value": r.AzAzureMonitorWorkspace},
				},
			},
		},
		nil,
	)

	if err != nil {
		log.Error(err, "failed BeginCreateOrUpdate", "deploymentName", deploymentName)
		return err
	}
	// TODO: Check the best practices here. I doubt we want to do this synchronously.
	// From my tests it usually takes less than 5s tho so might actually be ok.
	_, err = deploy.PollUntilDone(ctx, &azcoreruntime.PollUntilDoneOptions{Frequency: 5 * time.Second})
	if err != nil {
		return fmt.Errorf("cannot get the create deployment future respone: %v", err)
	}
	return nil
}
func (r *PromRuleToAzPromRuleGroupReconciler) deleteExternalResources(ctx context.Context, promRule monitoringv1.PrometheusRule) error {
	log := log.FromContext(ctx)
	annotations := promRule.GetAnnotations()
	resourceNames, ok := annotations[azPrometheusRuleGroupResourceNamesAnnotation]
	if ok {
		resourceNamesSplitted := strings.Split(resourceNames, ",")
		for _, rn := range resourceNamesSplitted {
			_, err := r.deletePrometheusRuleGroup(ctx, rn)
			if err != nil {
				log.Error(err, "Failed to delete prometeheus rule group", "resourceName", rn)
				// TODO: Should we try to delete the rest in case one deletion fails? Or simply retry again?
				return err
			}
		}
	}
	return nil
}

func (r *PromRuleToAzPromRuleGroupReconciler) deletePrometheusRuleGroup(ctx context.Context, ruleGroupName string) (*armalertsmanagement.PrometheusRuleGroupsClientDeleteResponse, error) {
	log := log.FromContext(ctx)
	resp, err := r.PrometheusRuleGroupsClient.Delete(ctx, r.AzResourceGroupName, ruleGroupName, nil)

	if err != nil {
		log.Error(err, "failed to delete the prometheus rule group", "ruleGroupName", ruleGroupName)
		return nil, err
	}
	log.Info("Sucessfully deleted PrometheusRuleGroup", "ruleGroupName", ruleGroupName)
	return &resp, nil
}

// func (r *PromRuleToAzPromRuleGroupReconciler) generateArmTemplateFromPromRule(ctx context.Context, promRule monitoringv1.PrometheusRule) (*armalertsmanagement.PrometheusRuleGroupResource, string, error) {
func (r *PromRuleToAzPromRuleGroupReconciler) generateArmTemplateFromPromRule(ctx context.Context, promRule monitoringv1.PrometheusRule) (string, error) {
	log := log.FromContext(ctx)
	// TODO: I have this working as well with the changes I proposed on the azure tool.
	// It's currently using exec to call the tool since I'm running it locally.
	// If we go with calling a node app, we can probably use something like https://github.com/rogchap/v8go
	// Or, we could re-write the tool in go if the azure maintainers are ok with it.

	for _, ruleGroup := range promRule.Spec.Groups {
		interval, err := prometheusmodel.ParseDuration(string(*ruleGroup.Interval))
		if err != nil {
			log.Error(err, "Failed to parse the Interval from the PrometheusRule Spec")
			return "", err
		}
		// Can't be lower than 1m.
		if interval < prometheusmodel.Duration(1*time.Minute) {
			*ruleGroup.Interval = monitoringv1.Duration("1m")
		}
	}

	marshalledPromRule, err := json.Marshal(promRule.Spec)

	if err != nil {
		log.Error(err, "Failed to marshal the promRule")
		return "", err
	}

	cmd := exec.Command(r.NodePath, r.AzPromRulesConverterPath, "-amw", "altinn-monitor-test-amw", "-l", "norwayeast", "-c", r.AzClusterName, "-j", string(marshalledPromRule))

	var out strings.Builder
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		log.Error(err, "Failed to convert PrometheusRule into PrometheusRuleGroup")
		return "", err
	}
	jsonString := out.String()

	return jsonString, nil
}

func timestamp() string {
	now := time.Now()

	var sb strings.Builder
	sb.WriteString(strconv.Itoa(now.Year()))
	sb.WriteString(strconv.Itoa(int(now.Month())))
	sb.WriteString(strconv.Itoa(now.Day()))
	sb.WriteString(strconv.Itoa(now.Hour()))
	sb.WriteString(strconv.Itoa(now.Minute()))
	sb.WriteString(strconv.Itoa(now.Second()))

	return sb.String()
}

// TODO: This is likely not needed. In the beginning I wasn't sure which annotations would be essential and which ones would be nice to haves.
func hasAllAnnotations(annotations map[string]string) bool {
	boolRes := true
	for _, a := range allAnnotations {
		_, ok := annotations[a]
		boolRes = boolRes && ok
	}
	return boolRes
}

func generateArmDeploymentName(req ctrl.Request, suffix string) string {
	// Limit of 64 characters
	return req.Namespace + "-" + req.Name + "-" + suffix
}
func removedGroups(old, new []string) []string {
	groupsToRemove := make([]string, 0)
	for _, viOld := range old {
		found := false
		for _, viNew := range new {
			if viNew == viOld {
				found = true
				break
			}
		}
		if !found {
			groupsToRemove = append(groupsToRemove, viOld)
		}
	}
	return groupsToRemove
}

func generateRuleGroupNamesAnnotationString(promRule monitoringv1.PrometheusRule) string {
	resourceNames := ""
	for idx, p := range promRule.Spec.Groups {
		if idx+1 < len(promRule.Spec.Groups) {
			resourceNames = resourceNames + p.Name + ","
		} else {
			resourceNames = resourceNames + p.Name
		}
	}
	return resourceNames
}

func hashArmTemplate(bytes []byte) string {
	h := sha256.New()
	h.Write(bytes)
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

// SetupWithManager sets up the controller with the Manager.
func (r *PromRuleToAzPromRuleGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Watches(
			&monitoringv1.PrometheusRule{},
			&handler.EnqueueRequestForObject{},
		).
		Named("promrule-to-azpromrulegroup").
		Complete(r)
}
