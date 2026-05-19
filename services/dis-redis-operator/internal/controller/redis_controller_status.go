package controller

import (
	"context"
	"fmt"

	redisv1alpha1 "github.com/Altinn/altinn-platform/services/dis-redis-operator/api/v1alpha1"
	redispkg "github.com/Altinn/altinn-platform/services/dis-redis-operator/internal/redis"
	cachev1 "github.com/Azure/azure-service-operator/v2/api/cache/v1api20250401"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *RedisReconciler) updateStatus(
	ctx context.Context,
	redisObj *redisv1alpha1.Redis,
	azureName string,
	identity redispkg.ResolvedIdentity,
	identityPending bool,
	cluster *cachev1.RedisEnterprise,
	clusterReady redispkg.ASOReadyCondition,
	database *cachev1.RedisEnterpriseDatabase,
	databaseReady redispkg.ASOReadyCondition,
	privateEndpointReady redispkg.ASOReadyCondition,
	privateDNSReady redispkg.ASOReadyCondition,
) error {
	updated := false

	applyCondition := func(c metav1.Condition) metav1.Condition {
		if setStatusCondition(redisObj, c) {
			updated = true
		}
		return c
	}

	identityCond := applyCondition(buildIdentityCondition(redisObj, identity))
	clusterCond := applyCondition(buildDependentCondition(
		redisObj.Generation,
		redisv1alpha1.ConditionClusterReady,
		identity,
		clusterReady,
		"ClusterNotReady",
		"waiting for ASO RedisEnterprise readiness",
	))
	databaseCond := applyCondition(buildDependentCondition(
		redisObj.Generation,
		redisv1alpha1.ConditionDatabaseReady,
		identity,
		databaseReady,
		"DatabaseNotReady",
		"waiting for ASO RedisEnterpriseDatabase readiness",
	))
	peCond := applyCondition(buildDependentCondition(
		redisObj.Generation,
		redisv1alpha1.ConditionPrivateEndpointReady,
		identity,
		privateEndpointReady,
		"PrivateEndpointNotReady",
		"waiting for ASO PrivateEndpoint readiness",
	))
	dnsCond := applyCondition(buildDependentCondition(
		redisObj.Generation,
		redisv1alpha1.ConditionPrivateDNSReady,
		identity,
		privateDNSReady,
		"PrivateDNSNotReady",
		"waiting for ASO shared private DNS zone readiness",
	))
	// AccessPolicyAssignment is deferred to a follow-up PR (ASO type pending in upstream).
	accessCond := applyCondition(redispkg.NewCondition(
		redisv1alpha1.ConditionAccessPolicyReady,
		redisObj.Generation,
		metav1.ConditionUnknown,
		"Pending",
		"access policy assignment is not yet implemented in this slice",
	))

	applyCondition(redispkg.AggregateReadyCondition(
		redisObj.Generation,
		identityCond,
		clusterCond,
		databaseCond,
		peCond,
		dnsCond,
		accessCond,
	))

	principalID := identity.PrincipalID
	if identityPending {
		principalID = ""
	}

	updated = setIfChanged(&redisObj.Status.AzureName, azureName) || updated
	updated = setIfChanged(&redisObj.Status.OwnerPrincipalID, principalID) || updated
	updated = setIfChanged(&redisObj.Status.ClusterResourceID, clusterResourceID(cluster)) || updated
	updated = setIfChanged(&redisObj.Status.DatabaseResourceID, databaseResourceID(database)) || updated
	updated = setIfChanged(&redisObj.Status.HostName, hostNameFromCluster(cluster)) || updated
	updated = setIfChanged(&redisObj.Status.Port, int32(redispkg.DefaultDatabasePort)) || updated
	updated = setIfChanged(&redisObj.Status.ObservedGeneration, redisObj.Generation) || updated

	if !updated {
		return nil
	}
	return r.Status().Update(ctx, redisObj)
}

func buildIdentityCondition(redisObj *redisv1alpha1.Redis, identity redispkg.ResolvedIdentity) metav1.Condition {
	if identity.IsPending() {
		return redispkg.NewCondition(
			redisv1alpha1.ConditionIdentityReady,
			redisObj.Generation,
			metav1.ConditionFalse,
			identity.PendingReason,
			identity.PendingMessage,
		)
	}
	return redispkg.NewCondition(
		redisv1alpha1.ConditionIdentityReady,
		redisObj.Generation,
		metav1.ConditionTrue,
		"IdentityReady",
		fmt.Sprintf("%s is ready", identity.SourceDescription()),
	)
}

func buildDependentCondition(
	generation int64,
	conditionType redisv1alpha1.ConditionType,
	identity redispkg.ResolvedIdentity,
	input redispkg.ASOReadyCondition,
	notReadyReason, notReadyMessage string,
) metav1.Condition {
	if identity.IsPending() {
		return redispkg.NewCondition(
			conditionType,
			generation,
			metav1.ConditionFalse,
			identity.PendingReason,
			fmt.Sprintf("waiting for owner identity before reconciling dependency: %s", identity.PendingMessage),
		)
	}
	return asoToStatusCondition(generation, conditionType, input, notReadyReason, notReadyMessage)
}

func asoToStatusCondition(
	generation int64,
	conditionType redisv1alpha1.ConditionType,
	input redispkg.ASOReadyCondition,
	notReadyReason, notReadyMessage string,
) metav1.Condition {
	if !input.Found {
		return redispkg.NewCondition(conditionType, generation, metav1.ConditionUnknown, "NotFound", "dependent resource not found")
	}

	reason := input.Reason
	if reason == "" {
		if input.Status == metav1.ConditionTrue {
			reason = "Ready"
		} else {
			reason = notReadyReason
		}
	}
	message := input.Message
	if message == "" {
		if input.Status == metav1.ConditionTrue {
			message = "dependency is ready"
		} else {
			message = notReadyMessage
		}
	}

	return redispkg.NewCondition(conditionType, generation, input.Status, reason, message)
}

func clusterResourceID(cluster *cachev1.RedisEnterprise) string {
	if cluster == nil || cluster.Status.Id == nil {
		return ""
	}
	return *cluster.Status.Id
}

func databaseResourceID(database *cachev1.RedisEnterpriseDatabase) string {
	if database == nil || database.Status.Id == nil {
		return ""
	}
	return *database.Status.Id
}

func hostNameFromCluster(cluster *cachev1.RedisEnterprise) string {
	if cluster == nil || cluster.Status.HostName == nil {
		return ""
	}
	return *cluster.Status.HostName
}

func setIfChanged[T comparable](field *T, value T) bool {
	if *field == value {
		return false
	}
	*field = value
	return true
}
