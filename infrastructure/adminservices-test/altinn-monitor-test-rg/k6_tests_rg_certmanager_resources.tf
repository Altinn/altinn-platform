# TODO: change to -production after testing and use prod server
resource "kubernetes_manifest" "letsencrypt_issuer" {
  manifest = yamldecode(<<EOT
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: letsencrypt-staging
  namespace: monitoring
spec:
  acme:
    # server: https://acme-v02.api.letsencrypt.org/directory
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    # Email address used for ACME registration
    # email: user@example.com
    profile: tlsserver
    privateKeySecretRef:
      name: letsencrypt-staging
    solvers:
      - http01:
          ingress:
            ingressClassName: nginx
  EOT
  )
}
