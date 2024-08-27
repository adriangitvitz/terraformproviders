package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubectlManifest() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubectlManifestCreate,
		ReadContext:   schema.NoopContext,
		UpdateContext: resourceKubectlManifestCreate,
		DeleteContext: resourceKubectlManifestDelete,
		Schema: map[string]*schema.Schema{
			"manifest_path": {
				Type:     schema.TypeString,
				Required: true,
			},
			"kustomize": {
				Type:     schema.TypeBool,
				Required: true,
			},
			"createns": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"namespace": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func resourceKubectlManifestCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	manifestPath := d.Get("manifest_path").(string)
	isKustomize := d.Get("kustomize").(bool)

	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return diag.FromErr(fmt.Errorf("manifest files does not exists at: %s", manifestPath))
	}

	absPath, err := filepath.Abs(manifestPath)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to get absolute path: %v", err))
	}

	if v, ok := d.GetOk("createns"); ok {
		namespaces := v.([]interface{})
		for _, nsdata := range namespaces {
			namespace := nsdata.(map[string]interface{})
			cmdns := exec.Command("kubectl", "create", "ns", namespace["namespace"].(string), "--save-config")
			out, err := cmdns.CombinedOutput()
			if err != nil {
				return diag.FromErr(fmt.Errorf("failed to apply manifest: %v\n%s", err, out))
			}
		}
	}

	var cmd *exec.Cmd
	if !isKustomize {
		cmd = exec.Command("kubectl", "apply", "-f", absPath)
	} else {
		cmd = exec.Command("kubectl", "apply", "-k", absPath)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to apply manifest: %v\n%s", err, out))
	}
	d.SetId(absPath)
	return diag.Diagnostics{}
}

func resourceKubectlManifestDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	manifestPath := d.Get("manifest_path").(string)
	isKustomize := d.Get("kustomize").(bool)

	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		d.SetId("")
		return diag.Diagnostics{}
	}

	absPath, err := filepath.Abs(manifestPath)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to get absolute path: %v", err))
	}

	var cmd *exec.Cmd
	if !isKustomize {
		cmd = exec.Command("kubectl", "delete", "-f", absPath)
	} else {
		cmd = exec.Command("kubectl", "delete", "-k", absPath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete manifest: %v\n%s", err, output))
	}

	d.SetId("")
	return diag.Diagnostics{}
}
