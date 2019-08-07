## banzai cluster feature dns

Manage cluster DNS feature

### Synopsis

Manage cluster DNS feature

```
banzai cluster feature dns [flags]
```

### Options

```
      --cluster int32         ID of cluster to manage DNS cluster feature of
      --cluster-name string   Name of cluster to manage DNS cluster feature of
  -h, --help                  help for dns
```

### Options inherited from parent commands

```
      --color                use colors on non-tty outputs
      --config string        config file (default is $BANZAICONFIG or $HOME/.banzai/config.yaml)
      --interactive          ask questions interactively even if stdin or stdout is non-tty
      --no-color             never display color output
      --no-interactive       never ask questions interactively
      --organization int32   organization id
  -o, --output string        output format (default|yaml|json) (default "default")
      --verbose              more verbose output
```

### SEE ALSO

* [banzai cluster feature](banzai_cluster_feature.md)	 - Manage cluster features
* [banzai cluster feature dns activate](banzai_cluster_feature_dns_activate.md)	 - Activate the DNS feature of a cluster
* [banzai cluster feature dns deactivate](banzai_cluster_feature_dns_deactivate.md)	 - Deactivate the DNS feature of a cluster
* [banzai cluster feature dns get](banzai_cluster_feature_dns_get.md)	 - Get details of the DNS feature for a cluster
* [banzai cluster feature dns update](banzai_cluster_feature_dns_update.md)	 - Update the DNS feature of a cluster
