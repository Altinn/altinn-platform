package controller

import (
	"github.com/Altinn/altinn-platform/services/dis-common/platformtags"
	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
)

// resourceTags returns the Azure tags for ASO resources created for the given
// database server: the platform finops tags (when the operator has base tags
// configured) plus the operator's dis-database marker tag.
func (r *DatabaseServerReconciler) resourceTags(db *storagev1alpha1.DatabaseServer) map[string]string {
	tags := platformtags.ForNamespace(r.Config.BaseTags, db.Namespace)
	if tags == nil {
		tags = make(map[string]string, 1)
	}
	tags[disDatabaseNamePrefix] = db.Name
	return tags
}
