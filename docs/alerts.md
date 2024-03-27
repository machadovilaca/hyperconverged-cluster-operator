# Hyperconverged Cluster Operator alerts

### HCOInstallationIncomplete
**Summary:** the installation was not completed; to complete the installation, create a HyperConverged custom resource.

**Description:** the installation was not completed; the HyperConverged custom resource is missing. In order to complete the installation of the Hyperconverged Cluster Operator you should create the HyperConverged custom resource.

**Severity:** info

**For:** 1h.

### KubeVirtCRModified
**Summary:** {{ $value }} out-of-band CR modifications were detected in the last 10 minutes.

**Description:** Out-of-band modification for {{ $labels.component_name }}.

**Severity:** warning

### SingleStackIPv6Unsupported
**Summary:** KubeVirt Hyperconverged is not supported on a single stack IPv6 cluster

**Description:** KubeVirt Hyperconverged is not supported on a single stack IPv6 cluster

**Severity:** critical

### UnsupportedHCOModification
**Summary:** {{ $value }} unsafe modifications were detected in the HyperConverged resource.

**Description:** unsafe modification for the {{ $labels.annotation_name }} annotation in the HyperConverged resource.

**Severity:** info

## Developing new alerts

All alerts documented here are auto-generated and reflect exactly what is being
exposed. After developing new alerts or changing old ones please regenerate
this document.
