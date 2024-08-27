package clusterkind

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
)

func resourceKindCluster() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKindClusterCreate,
		ReadContext:   resourceKindClusterRead,
		UpdateContext: resourceKindClusterUpdate,
		DeleteContext: resourceKindClusterDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				// ForceNew: true,
			},
			"containerd_config_patches": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"node": {
				Type:     schema.TypeList,
				Required: true,
				// ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"role": {
							Type:     schema.TypeString,
							Required: true,
						},
						"extra_mounts": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"host_path": {
										Type:     schema.TypeString,
										Required: true,
									},
									"container_path": {
										Type:     schema.TypeString,
										Required: true,
									},
								},
							},
						},
						"kube_adm_config_patches": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func resourceKindClusterCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Get("name").(string)
	config := &v1alpha4.Cluster{
		TypeMeta: v1alpha4.TypeMeta{
			APIVersion: "",
			Kind:       "Cluster",
		},
		Name: name,
	}

	if v, ok := d.GetOk("containerd_config_patches"); ok {
		patches := v.([]interface{})
		config.ContainerdConfigPatches = make([]string, len(patches))
		for i, patch := range patches {
			config.ContainerdConfigPatches[i] = patch.(string)
		}
	}

	if v, ok := d.GetOk("node"); ok {
		nodes := v.([]interface{})
		for _, nodeData := range nodes {
			node := nodeData.(map[string]interface{})
			kindNode := v1alpha4.Node{
				Role: v1alpha4.NodeRole(node["role"].(string)),
			}

			if extraMounts, ok := node["extra_mounts"].(map[string]interface{}); ok {
				for mountPath, mountData := range extraMounts {
					mountInfo := mountData.(map[string]interface{})
					kindNode.ExtraMounts = append(kindNode.ExtraMounts, v1alpha4.Mount{
						HostPath:      mountInfo["host_path"].(string),
						ContainerPath: mountPath,
					})
				}
			}
			if patches, ok := node["kube_adm_config_patches"].(string); ok {
				kindNode.KubeadmConfigPatches = []string{patches}
			}

			config.Nodes = append(config.Nodes, kindNode)
		}
	}
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(cmd.NewLogger()),
	)

	if err := provider.Create(
		name,
		cluster.CreateWithV1Alpha4Config(config),
	); err != nil {
		return diag.FromErr(fmt.Errorf("error creating cluster: %v", err))
	}

	d.SetId(name)

	if err := d.Set("containerd_config_patches", config.ContainerdConfigPatches); err != nil {
		return diag.FromErr(fmt.Errorf("error setting containerd_config_patches: %v", err))
	}

	nodes := make([]interface{}, len(config.Nodes))
	for i, node := range config.Nodes {
		nodeMap := map[string]interface{}{
			"role": string(node.Role),
		}
		if len(node.KubeadmConfigPatches) > 0 {
			nodeMap["kube_adm_config_patches"] = node.KubeadmConfigPatches[0]
		}
		if len(node.ExtraMounts) > 0 {
			extraMounts := make(map[string]interface{})
			for _, mount := range node.ExtraMounts {
				extraMounts[mount.ContainerPath] = map[string]interface{}{
					"host_path": mount.HostPath,
				}
			}
			nodeMap["extra_mounts"] = extraMounts
		}
		nodes[i] = nodeMap
	}
	if err := d.Set("node", nodes); err != nil {
		return diag.FromErr(fmt.Errorf("error setting node config: %v", err))
	}
	return resourceKindClusterRead(ctx, d, meta)
}

func resourceKindClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Id()

	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(cmd.NewLogger()),
	)
	exists, err := provider.List()
	if err != nil {
		return diag.FromErr(fmt.Errorf("error listing clusters: %v", err))
	}

	clusterExists := false
	for _, clusterName := range exists {
		if clusterName == name {
			clusterExists = true
			break
		}
	}

	if !clusterExists {
		d.SetId("")
		return nil
	}

	nodes, err := provider.ListNodes(name)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error listing nodes for cluster %s: %v", name, err))
	}

	nodeConfigs := make([]interface{}, len(nodes))
	for i, node := range nodes {
		nodeConfig := make(map[string]interface{})

		role, err := node.Role()
		if err != nil {
			return diag.FromErr(err)
		}
		if role == "control-plane" {
			nodeConfig["role"] = "control-plane"
		} else {
			nodeConfig["role"] = "worker"
		}

		nodeConfigs[i] = nodeConfig
	}

	if err := d.Set("node", nodeConfigs); err != nil {
		return diag.FromErr(fmt.Errorf("error setting node config: %v", err))
	}

	return nil
}

func resourceKindClusterUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if err := resourceKindClusterDelete(ctx, d, meta); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting existing cluster during update: %v", err))
	}

	return resourceKindClusterCreate(ctx, d, meta)
}

func resourceKindClusterDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Id()

	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(cmd.NewLogger()),
	)

	if err := provider.Delete(name, ""); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting cluster %s: %v", name, err))
	}

	d.SetId("")
	return nil
}
