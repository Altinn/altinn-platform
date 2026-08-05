// Flux records the origin git repo + commit of a source artifact as OCI
// annotations on the *source* object (OCIRepository/GitRepository), under
// status.artifact.metadata. A Kustomization/HelmRelease only references the
// source via spec.sourceRef, so a "view source" link is resolved by reading the
// source object's annotations (see useSourceLink).

const ANNO_SOURCE = 'org.opencontainers.image.source';
const ANNO_REVISION = 'org.opencontainers.image.revision';

export interface SourceLink {
  /** Repo URL, e.g. https://github.com/dis-way/gitops-manifests */
  repoUrl: string;
  /** org/repo, e.g. dis-way/gitops-manifests */
  label: string;
  ref?: string;
  sha?: string;
  shortSha?: string;
  /** Repo at the exact commit, when the sha is known. */
  commitUrl?: string;
}

export interface SourceRef {
  kind: string;
  name: string;
  namespace?: string;
}

function asRecord(v: unknown): Record<string, unknown> | undefined {
  return v && typeof v === 'object' ? (v as Record<string, unknown>) : undefined;
}

function repoLabel(url: string): string {
  try {
    return new URL(url).pathname.replace(/^\/+/, '').replace(/\.git$/, '') || url;
  } catch {
    return url;
  }
}

/**
 * Parse Flux's `org.opencontainers.image.revision` into a ref + commit sha.
 * Seen formats: `main/<sha>` and `<branch>@sha1:<sha>`.
 */
export function parseOciRevision(rev: string | undefined): { ref?: string; sha?: string } {
  if (!rev) return {};
  const sha = rev.match(/sha\d*:([0-9a-f]{7,})/i)?.[1];
  if (sha) return { ref: rev.split('@')[0] || undefined, sha };
  if (rev.includes('/')) {
    const i = rev.lastIndexOf('/');
    return { ref: rev.slice(0, i) || undefined, sha: rev.slice(i + 1) || undefined };
  }
  if (/^[0-9a-f]{7,}$/i.test(rev)) return { sha: rev };
  return { ref: rev };
}

/** Build a source link from the origin annotations' values, or null. */
export function linkFromOrigin(source?: string, revision?: string): SourceLink | null {
  if (!source || !/^https?:\/\//.test(source)) return null;
  const repoUrl = source.replace(/\.git$/, '');
  const { ref, sha } = parseOciRevision(revision);
  const link: SourceLink = { repoUrl, label: repoLabel(repoUrl), ref, sha, shortSha: sha?.slice(0, 7) };
  if (sha) link.commitUrl = `${repoUrl}/commit/${sha}`;
  return link;
}

/**
 * A git commit URL for a deployed-revision string (lastAppliedRevision /
 * status-history revisions). Only revisions that embed a git sha link:
 * `main@sha1:<sha>`, `<branch>/<sha>`, or a bare sha. OCI digests
 * (`tag@sha256:…`) are artifact versions, not commits, and chart versions
 * (`1.4.7`) have no commit at all.
 */
export function commitFromRevision(
  repoUrl: string | undefined,
  revision: string | undefined,
): string | null {
  if (!repoUrl || !/^https?:\/\//.test(repoUrl) || !revision) return null;
  const repo = repoUrl.replace(/\.git$/, '');
  const sha1 = revision.match(/(?:^|@)sha1:([0-9a-f]{7,40})\b/i)?.[1];
  if (sha1) return `${repo}/commit/${sha1}`;
  if (/@sha\d+:/i.test(revision)) return null;
  // Charts built from a GitRepository (reconcileStrategy: Revision) embed the
  // sha as semver build metadata, e.g. `1.2.3+ab12cd34`.
  const build = revision.match(/\+([0-9a-f]{7,40})$/i)?.[1];
  if (build) return `${repo}/commit/${build}`;
  const tail = revision.includes('/') ? revision.slice(revision.lastIndexOf('/') + 1) : revision;
  return /^[0-9a-f]{7,40}$/i.test(tail) ? `${repo}/commit/${tail}` : null;
}

/** The origin fields of a Flux source object, as the fleet API projects them. */
export interface OriginSource {
  revision?: string;
  originSource?: string;
  originRevision?: string;
}

/**
 * Commit URL for one release of an app: a git-sha revision links directly;
 * an OCI digest links only if a source object is currently fetching that
 * digest (its origin annotations name the commit) — older digests' commits
 * are unknowable from the digest alone.
 */
export function releaseCommitUrl(
  revision: string | undefined,
  repoUrl: string | undefined,
  sources: OriginSource[] = [],
): string | null {
  const direct = commitFromRevision(repoUrl, revision);
  if (direct) return direct;
  const digest = revision?.match(/@sha256:([0-9a-f]+)/i)?.[1];
  if (!digest) return null;
  for (const s of sources) {
    if (s.revision?.includes(digest)) {
      const url = linkFromOrigin(s.originSource, s.originRevision)?.commitUrl;
      if (url) return url;
    }
  }
  return null;
}

/** Extract a source link from a source object's raw (OCIRepository/...), or null. */
export function sourceLinkFromRaw(raw: unknown): SourceLink | null {
  const status = asRecord(asRecord(raw)?.status);
  const artifact = asRecord(status?.artifact);
  const meta = asRecord(artifact?.metadata);
  const source = meta?.[ANNO_SOURCE];
  if (typeof source !== 'string') return null;
  const revision = typeof meta?.[ANNO_REVISION] === 'string' ? (meta[ANNO_REVISION] as string) : undefined;
  return linkFromOrigin(source, revision);
}

export interface AppliedBy {
  name: string;
  namespace?: string;
}

/** The Kustomization that applied this object, from Flux's kustomize labels. */
export function appliedByFromRaw(raw: unknown): AppliedBy | null {
  const labels = asRecord(asRecord(asRecord(raw)?.metadata)?.labels);
  const name = labels?.['kustomize.toolkit.fluxcd.io/name'];
  if (typeof name !== 'string' || !name) return null;
  const namespace = labels?.['kustomize.toolkit.fluxcd.io/namespace'];
  return { name, namespace: typeof namespace === 'string' ? namespace : undefined };
}

export interface OwnerRef {
  kind: string;
  name: string;
}

/**
 * The controlling owner from the raw object's ownerReferences — the direct
 * creator when a controller made this object (an otel-operator collector
 * Deployment is owned by its OpenTelemetryCollector CR; azapi root objects
 * are owned by a FluxConfig CR). Falls back to the first reference when none
 * is marked controller.
 */
export function ownerFromRaw(raw: unknown): OwnerRef | null {
  const meta = asRecord(asRecord(raw)?.metadata);
  const refs = meta?.ownerReferences;
  if (!Array.isArray(refs)) return null;
  const recs = refs.map(asRecord).filter((r): r is Record<string, unknown> => Boolean(r));
  const pick = recs.find((r) => r.controller === true) ?? recs[0];
  if (!pick || typeof pick.kind !== 'string' || typeof pick.name !== 'string') return null;
  return { kind: pick.kind, name: pick.name };
}

/**
 * What KIND of object applied this one. The projected `appliedBy` carries only
 * name + namespace; the raw object says which deployer wrote it — kustomize
 * owner labels name a Kustomization, Helm's managed-by label (with the
 * meta.helm.sh release annotations) names a HelmRelease.
 */
export function applierKindFromRaw(raw: unknown): 'Kustomization' | 'HelmRelease' | null {
  const meta = asRecord(asRecord(raw)?.metadata);
  const labels = asRecord(meta?.labels);
  if (typeof labels?.['kustomize.toolkit.fluxcd.io/name'] === 'string') return 'Kustomization';
  if (labels?.['app.kubernetes.io/managed-by'] === 'Helm') return 'HelmRelease';
  return null;
}

export interface ManagedBy {
  name: string;
  namespace?: string;
}

/**
 * The Azure fluxConfiguration that manages this object. The AKS GitOps
 * extension stamps clusterconfig.azure.com/{name,namespace,is-managed} labels
 * on the root OCIRepository/Kustomization pairs it creates (the objects
 * provisioned via azapi fluxConfigurations) — the provenance for roots, which
 * carry no kustomize appliedBy labels.
 */
export function managedByFromRaw(raw: unknown): ManagedBy | null {
  const labels = asRecord(asRecord(asRecord(raw)?.metadata)?.labels);
  const name = labels?.['clusterconfig.azure.com/name'];
  if (typeof name !== 'string' || !name) return null;
  const namespace = labels?.['clusterconfig.azure.com/namespace'];
  return { name, namespace: typeof namespace === 'string' ? namespace : undefined };
}

/** Read spec.sourceRef from a Kustomization/HelmRelease raw, or null. */
export function sourceRefFromRaw(raw: unknown): SourceRef | null {
  const ref = asRecord(asRecord(asRecord(raw)?.spec)?.sourceRef);
  if (!ref || typeof ref.kind !== 'string' || typeof ref.name !== 'string') return null;
  return {
    kind: ref.kind,
    name: ref.name,
    namespace: typeof ref.namespace === 'string' ? ref.namespace : undefined,
  };
}
