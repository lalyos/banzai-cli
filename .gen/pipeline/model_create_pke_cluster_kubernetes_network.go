/*
 * Pipeline API
 *
 * Pipeline v0.3.0 swagger
 *
 * API version: 0.21.2
 * Contact: info@banzaicloud.com
 */

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package pipeline

type CreatePkeClusterKubernetesNetwork struct {
	ServiceCIDR string `json:"serviceCIDR,omitempty"`
	PodCIDR string `json:"podCIDR,omitempty"`
	Provider string `json:"provider,omitempty"`
	ProviderConfig map[string]interface{} `json:"providerConfig,omitempty"`
}