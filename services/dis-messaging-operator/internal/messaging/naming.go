package messaging

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// PrefixAnnotation on a namespace sets the queue name prefix of that
	// namespace. It overrides the default prefix of a product namespace.
	PrefixAnnotation = "messaging.dis.altinn.cloud/prefix"

	// ReasonInvalidName is the Ready condition reason for a Queue that fails
	// ValidateName.
	ReasonInvalidName = "InvalidName"

	productNamespacePrefix = "product-"
	maxQueueNameLength     = 260
)

// prefixPattern allows the characters of an Azure queue name. A prefix ends
// with a separator, so that a prefix "team-a" cannot match the queues of the
// prefix "team-ab.".
var prefixPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*[._/-]$`)

// Prefix returns the queue name prefix of a namespace. PrefixAnnotation wins
// when it is set. Otherwise the namespace "product-<name>" has the prefix
// "<name>.". Any other namespace has no prefix, and Prefix returns an error.
func Prefix(namespace string, annotations map[string]string) (string, error) {
	if prefix, ok := annotations[PrefixAnnotation]; ok {
		if len(prefix) >= maxQueueNameLength || !prefixPattern.MatchString(prefix) {
			return "", fmt.Errorf("namespace %s has an invalid %s annotation %q: "+
				"use letters, numbers, '.', '_', '-' and '/', start with a letter or number and end with '.', '_', '-' or '/'",
				namespace, PrefixAnnotation, prefix)
		}
		return prefix, nil
	}

	product, ok := strings.CutPrefix(namespace, productNamespacePrefix)
	if !ok || product == "" {
		return "", fmt.Errorf("namespace %s has no queue name prefix: it is not a %s<name> namespace and has no %s annotation",
			namespace, productNamespacePrefix, PrefixAnnotation)
	}
	return product + ".", nil
}

// ValidateName checks that a queue name is the prefix of its namespace
// followed by at least one character. The check is case-sensitive. Every
// error sets Ready=False with reason ReasonInvalidName on the Queue.
func ValidateName(namespace string, annotations map[string]string, name string) error {
	prefix, err := Prefix(namespace, annotations)
	if err != nil {
		return err
	}
	if rest, ok := strings.CutPrefix(name, prefix); !ok || rest == "" {
		return fmt.Errorf("queue name %q must start with %q, the prefix of namespace %s, and continue after it",
			name, prefix, namespace)
	}
	return nil
}
