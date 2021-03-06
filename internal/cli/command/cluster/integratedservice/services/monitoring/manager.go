// Copyright © 2019 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package monitoring

import (
	"context"
	"fmt"
	"strconv"

	"emperror.dev/errors"
	"github.com/antihax/optional"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"

	"github.com/banzaicloud/banzai-cli/.gen/pipeline"
	"github.com/banzaicloud/banzai-cli/internal/cli"
	clustercontext "github.com/banzaicloud/banzai-cli/internal/cli/command/cluster/context"
	"github.com/banzaicloud/banzai-cli/internal/cli/input"
)

type Manager struct {
	banzaiCLI cli.Cli
}

func NewManager(banzaiCLI cli.Cli) Manager {
	return Manager{
		banzaiCLI: banzaiCLI,
	}
}

func (Manager) ReadableName() string {
	return "Monitoring"
}

func (Manager) ServiceName() string {
	return "monitoring"
}

func (m Manager) BuildActivateRequestInteractively(clusterCtx clustercontext.Context) (pipeline.ActivateIntegratedServiceRequest, error) {

	grafana, err := askGrafana(m.banzaiCLI, grafanaSpec{
		Enabled:    true,
		Dashboards: true,
		Ingress: baseIngressSpec{
			Enabled: true,
			Path:    "/grafana",
		},
	})
	if err != nil {
		return pipeline.ActivateIntegratedServiceRequest{}, errors.WrapIf(err, "error during getting Grafana options")
	}

	prometheus, err := askPrometheus(m.banzaiCLI, prometheusSpec{
		Enabled: true,
		Storage: storageSpec{
			Size:      100,
			Retention: "10d",
		},
		Ingress: ingressSpecWithSecret{
			baseIngressSpec: baseIngressSpec{
				Enabled: true,
				Path:    "/prometheus",
			},
		},
	})
	if err != nil {
		return pipeline.ActivateIntegratedServiceRequest{}, errors.WrapIf(err, "error during getting Prometheus options")
	}

	alertmanager, err := askAlertmanager(m.banzaiCLI, alertmanagerSpec{
		Enabled: true,
		Ingress: ingressSpecWithSecret{
			baseIngressSpec: baseIngressSpec{
				Enabled: true,
				Path:    "/alertmanager",
			},
		},
		Provider: map[string]interface{}{
			alertmanagerProviderSlack: slackSpec{
				Enabled:      false,
				SendResolved: true,
			},
			alertmanagerProviderPagerDuty: pagerDutySpec{
				Enabled:      false,
				SendResolved: true,
			},
		},
	})
	if err != nil {
		return pipeline.ActivateIntegratedServiceRequest{}, errors.WrapIf(err, "error during getting Alertmanager options")
	}

	pushgateway, err := askPushgateway(m.banzaiCLI, pushgatewaySpec{
		Enabled: true,
		Ingress: ingressSpecWithSecret{
			baseIngressSpec: baseIngressSpec{
				Enabled: false,
				Path:    "/pushgateway",
			},
		},
	})
	if err != nil {
		return pipeline.ActivateIntegratedServiceRequest{}, errors.WrapIf(err, "error during getting Pushgateway options")
	}

	return pipeline.ActivateIntegratedServiceRequest{
		Spec: map[string]interface{}{
			"grafana":      grafana,
			"prometheus":   prometheus,
			"alertmanager": alertmanager,
			"pushgateway":  pushgateway,
			"exporters": exportersSpec{
				Enabled: true,
				NodeExporter: exporterBaseSpec{
					Enabled: true,
				},
				KubeStateMetrics: exporterBaseSpec{
					Enabled: true,
				},
			},
		},
	}, nil
}

func (m Manager) BuildUpdateRequestInteractively(clusterCtx clustercontext.Context, request *pipeline.UpdateIntegratedServiceRequest) error {

	var spec serviceSpec
	if err := mapstructure.Decode(request.Spec, &spec); err != nil {
		return errors.WrapIf(err, "service specification does not conform to schema")
	}

	grafana, err := askGrafana(m.banzaiCLI, spec.Grafana)
	if err != nil {
		return errors.WrapIf(err, "error during getting Grafana options")
	}

	prometheus, err := askPrometheus(m.banzaiCLI, spec.Prometheus)
	if err != nil {
		return errors.WrapIf(err, "error during getting Prometheus options")
	}

	alertmanager, err := askAlertmanager(m.banzaiCLI, spec.Alertmanager)
	if err != nil {
		return errors.WrapIf(err, "error during getting Alertmanager options")
	}

	pushgateway, err := askPushgateway(m.banzaiCLI, spec.Pushgateway)
	if err != nil {
		return errors.WrapIf(err, "error during getting Pushgateway options")
	}

	request.Spec["grafana"] = grafana
	request.Spec["prometheus"] = prometheus
	request.Spec["alertmanager"] = alertmanager
	request.Spec["pushgateway"] = pushgateway

	return nil
}

func (Manager) ValidateSpec(specObj map[string]interface{}) error {
	var spec serviceSpec

	if err := mapstructure.Decode(specObj, &spec); err != nil {
		return errors.WrapIf(err, "service specification does not conform to schema")
	}

	return spec.Validate()
}

type baseOutputItems struct {
	Url        string `mapstructure:"url"`
	SecretID   string `mapstructure:"secretId"`
	Version    string `mapstructure:"version"`
	ServiceURL string `mapstructure:"serviceUrl"`
}

type outputResponse struct {
	Alertmanager struct {
		baseOutputItems `mapstructure:",squash"`
	} `mapstructure:"alertmanager"`
	Grafana struct {
		baseOutputItems `mapstructure:",squash"`
	} `mapstructure:"grafana"`
	Prometheus struct {
		baseOutputItems `mapstructure:",squash"`
	} `mapstructure:"prometheus"`
	PrometheusOperator struct {
		Version string `mapstructure:"version"`
	} `mapstructure:"prometheusOperator"`
	Pushgateway struct {
		baseOutputItems `mapstructure:",squash"`
	} `mapstructure:"pushgateway"`
}

type TableData map[string]interface{}

func (Manager) WriteDetailsTable(details pipeline.IntegratedServiceDetails) map[string]map[string]interface{} {
	tableData := map[string]map[string]interface{}{
		"Monitoring": {
			"Status": details.Status,
		},
	}

	if details.Status == "INACTIVE" {
		return tableData
	}

	var output outputResponse
	if err := mapstructure.Decode(details.Output, &output); err != nil {
		log.Errorf("failed to unmarshal output: %s", err.Error())
		return tableData
	}

	var spec serviceSpec
	if err := mapstructure.Decode(details.Spec, &spec); err != nil {
		log.Errorf("failed to unmarshal output: %s", err.Error())
		return tableData
	}

	// Alertmanager outputs
	if spec.Alertmanager.Enabled {
		var secretID string
		if spec.Alertmanager.Ingress.Enabled {
			secretID = spec.Alertmanager.Ingress.SecretId
			if secretID == "" {
				secretID = output.Alertmanager.SecretID
			}
		}
		var alertmanagerTable = TableData{
			"url":        output.Alertmanager.Url,
			"version":    output.Alertmanager.Version,
			"serviceUrl": output.Alertmanager.ServiceURL,
			"secretID":   secretID,
			"path":       spec.Alertmanager.Ingress.Path,
			"domain":     spec.Alertmanager.Ingress.Domain,
		}
		tableData["Alertmanager"] = alertmanagerTable
		// todo (colin): add provider outputs
	}

	// Grafana outputs
	if spec.Grafana.Enabled {
		var secretID = spec.Grafana.SecretId
		if secretID == "" {
			secretID = output.Grafana.SecretID
		}
		var grafanaTable = TableData{
			"url":        output.Grafana.Url,
			"version":    output.Grafana.Version,
			"serviceUrl": output.Grafana.ServiceURL,
			"secretID":   secretID,
			"path":       spec.Grafana.Ingress.Path,
			"domain":     spec.Grafana.Ingress.Domain,
		}
		tableData["Grafana"] = grafanaTable
	}

	// Prometheus outputs
	if spec.Prometheus.Enabled {
		var secretID string
		if spec.Prometheus.Ingress.Enabled {
			secretID = spec.Prometheus.Ingress.SecretId
			if secretID == "" {
				secretID = output.Prometheus.SecretID
			}
		}
		var prometheusTable = TableData{
			"url":        output.Prometheus.Url,
			"version":    output.Prometheus.Version,
			"serviceUrl": output.Prometheus.ServiceURL,
			"secretID":   secretID,
			"path":       spec.Prometheus.Ingress.Path,
			"domain":     spec.Prometheus.Ingress.Domain,
		}
		tableData["Prometheus"] = prometheusTable

		tableData["Prometheus_storage"] = TableData{
			"class":     spec.Prometheus.Storage.Class,
			"size":      spec.Prometheus.Storage.Size,
			"retention": spec.Prometheus.Storage.Retention,
		}
	}

	if spec.Pushgateway.Enabled {
		var secretID string
		if spec.Pushgateway.Ingress.Enabled {
			secretID = spec.Pushgateway.Ingress.SecretId
			if secretID == "" {
				secretID = output.Pushgateway.SecretID
			}
		}
		var pushgatewayTable = TableData{
			"url":        output.Pushgateway.Url,
			"version":    output.Pushgateway.Version,
			"serviceUrl": output.Pushgateway.ServiceURL,
			"secretID":   secretID,
			"path":       spec.Pushgateway.Ingress.Path,
			"domain":     spec.Pushgateway.Ingress.Domain,
		}
		tableData["Pushgateway"] = pushgatewayTable
	}

	if spec.Exporters.Enabled {
		tableData["Exporters"] = TableData{
			"nodeExporter":     spec.Exporters.NodeExporter.Enabled,
			"kubeStateMetrics": spec.Exporters.KubeStateMetrics.Enabled,
		}
	}

	tableData["Prometheus_operator"] = TableData{
		"version": output.PrometheusOperator.Version,
	}

	return tableData
}

func askIngress(compType string, defaults baseIngressSpec) (*baseIngressSpec, error) {
	var isIngressEnabled bool
	var domain string
	var path string

	if err := input.DoQuestions([]input.QuestionMaker{
		input.QuestionConfirm{
			QuestionBase: input.QuestionBase{
				Message: fmt.Sprintf("Do you want to enable %s Ingress?", compType),
			},
			DefaultValue: defaults.Enabled,
			Output:       &isIngressEnabled,
		},
	}); err != nil {
		return nil, errors.WrapIf(err, fmt.Sprintf("error during getting %s ingress enabled", compType))
	}

	if isIngressEnabled {
		var questions = []input.QuestionMaker{
			input.QuestionInput{
				QuestionBase: input.QuestionBase{
					Message: fmt.Sprintf("Please provide %s Ingress domain:", compType),
					Help:    "Leave empty to use cluster's IP",
				},
				DefaultValue: defaults.Domain,
				Output:       &domain,
			},
			input.QuestionInput{
				QuestionBase: input.QuestionBase{
					Message: fmt.Sprintf("Please provide %s Ingress path:", compType),
				},
				DefaultValue: defaults.Path,
				Output:       &path,
			},
		}
		if err := input.DoQuestions(questions); err != nil {
			return nil, errors.WrapIf(err, "error during asking ingress fields")
		}

	}
	return &baseIngressSpec{
		Enabled: isIngressEnabled,
		Domain:  domain,
		Path:    path,
	}, nil
}

func askGrafana(banzaiCLI cli.Cli, defaults grafanaSpec) (*grafanaSpec, error) {
	var isEnabled bool
	if err := input.DoQuestions([]input.QuestionMaker{
		input.QuestionConfirm{
			QuestionBase: input.QuestionBase{
				Message: "Do you want to enable Grafana?",
			},
			DefaultValue: defaults.Enabled,
			Output:       &isEnabled,
		},
	}); err != nil {
		return nil, errors.WrapIf(err, "error during getting Grafana enabled")
	}

	var result = &grafanaSpec{
		Enabled: isEnabled,
	}
	if isEnabled {
		var err error
		// secret
		result.SecretId, err = askSecret(banzaiCLI, passwordSecretType, defaults.SecretId, true)
		if err != nil {
			return nil, errors.WrapIf(err, "error during getting Grafana secret")
		}

		// ingress
		ingressSpec, err := askIngress("Grafana", defaults.Ingress)
		if err != nil {
			return nil, errors.WrapIf(err, "error during getting Grafana ingress options")
		}
		result.Ingress = *ingressSpec

		// default dashboards
		if err := input.DoQuestions([]input.QuestionMaker{
			input.QuestionConfirm{
				QuestionBase: input.QuestionBase{
					Message: "Do you want to add default dashboards to Grafana?",
				},
				DefaultValue: defaults.Dashboards,
				Output:       &result.Dashboards,
			},
		}); err != nil {
			return nil, errors.WrapIf(err, "error during getting default dashboards")
		}
	}

	return result, nil
}

func askPrometheus(banzaiCLI cli.Cli, defaults prometheusSpec) (*prometheusSpec, error) {
	var result = &prometheusSpec{
		Enabled: true,
	}

	// storage class, storage size and retention
	var storageSize = fmt.Sprint(defaults.Storage.Size)
	if err := input.DoQuestions([]input.QuestionMaker{
		input.QuestionInput{
			QuestionBase: input.QuestionBase{
				Message: "Please provide storage class name for Prometheus:",
				Help:    "Leave empty to use default storage class",
			},
			DefaultValue: defaults.Storage.Class,
			Output:       &result.Storage.Class,
		},
		input.QuestionInput{
			QuestionBase: input.QuestionBase{
				Message: "Please provide storage size for Prometheus:",
			},
			DefaultValue: storageSize,
			Output:       &storageSize,
		},
		input.QuestionInput{
			QuestionBase: input.QuestionBase{
				Message: "Please provide retention for Prometheus:",
			},
			DefaultValue: defaults.Storage.Retention,
			Output:       &result.Storage.Retention,
		},
	}); err != nil {
		return nil, errors.WrapIf(err, "error during getting Prometheus options")
	}

	storageSizeInt, err := strconv.ParseUint(storageSize, 10, 64)
	if err != nil {
		return nil, errors.WrapIf(err, "failed to parse storage size")
	}
	result.Storage.Size = uint(storageSizeInt)

	// ingress
	ingressSpec, err := askIngress("Prometheus", defaults.Ingress.baseIngressSpec)
	if err != nil {
		return nil, errors.WrapIf(err, "error during getting Prometheus ingress options")
	}
	result.Ingress.baseIngressSpec = *ingressSpec

	if ingressSpec.Enabled {
		result.Ingress.SecretId, err = askSecret(banzaiCLI, htPasswordSecretType, defaults.Ingress.SecretId, true)
		if err != nil {
			return nil, errors.WrapIf(err, "error during getting secret for Prometheus ingress")
		}
	}

	return result, nil
}

func askAlertmanager(banzaiCLI cli.Cli, defaults alertmanagerSpec) (*alertmanagerSpec, error) {
	var isEnabled bool
	if err := input.DoQuestions([]input.QuestionMaker{
		input.QuestionConfirm{
			QuestionBase: input.QuestionBase{
				Message: "Do you want to enable Alertmanager?",
			},
			DefaultValue: defaults.Enabled,
			Output:       &isEnabled,
		},
	}); err != nil {
		return nil, errors.WrapIf(err, "error during getting Alertmanager enabled")
	}

	var result = &alertmanagerSpec{
		Enabled: isEnabled,
	}

	if isEnabled {
		result.Provider = map[string]interface{}{
			alertmanagerProviderSlack: slackSpec{
				Enabled: false,
			},
			alertmanagerProviderPagerDuty: pagerDutySpec{
				Enabled: false,
			},
		}

		// ask provider options
		const skip = "skip"
		var notificationProvider string
		var defaultNotificationProviderValue = skip
		if pdProp, ok := defaults.Provider[alertmanagerProviderPagerDuty]; ok {
			var pd pagerDutySpec
			if err := mapstructure.Decode(pdProp, &pd); err == nil {
				if pd.Enabled {
					defaultNotificationProviderValue = alertmanagerNotificationNamePagerDuty
				}
			}
		} else if slackProp, ok := defaults.Provider[alertmanagerProviderSlack]; ok {
			var slack slackSpec
			if err := mapstructure.Decode(slackProp, &slack); err == nil {
				if slack.Enabled {
					defaultNotificationProviderValue = alertmanagerNotificationNameSlack
				}
			}
		}
		if err := input.DoQuestions([]input.QuestionMaker{
			input.QuestionSelect{
				QuestionInput: input.QuestionInput{
					QuestionBase: input.QuestionBase{
						Message: "Select notification provider",
					},
					DefaultValue: defaultNotificationProviderValue,
					Output:       &notificationProvider,
				},
				Options: []string{skip, alertmanagerNotificationNameSlack, alertmanagerNotificationNamePagerDuty},
			},
		}); err != nil {
			return nil, errors.WrapIf(err, "error during getting notification provider")
		}

		var err error
		switch notificationProvider {
		case alertmanagerNotificationNameSlack:
			result.Provider[alertmanagerProviderSlack], err = askNotificationProviderSlack(banzaiCLI, defaults.Provider[alertmanagerProviderSlack])
			if err != nil {
				return nil, errors.WrapIf(err, "error during getting Slack provider options")
			}
		case alertmanagerNotificationNamePagerDuty:
			result.Provider[alertmanagerProviderPagerDuty], err = askNotificationProviderPagerDuty(banzaiCLI, defaults.Provider[alertmanagerProviderPagerDuty])
			if err != nil {
				return nil, errors.WrapIf(err, "error during getting PagerDuty provider options")
			}
		case skip:
		default:
			return nil, errors.NewWithDetails("not supported provider type", "provider", notificationProvider)
		}

		// ask ingress
		ingressSpec, err := askIngress("Alertmanager", defaults.Ingress.baseIngressSpec)
		if err != nil {
			return nil, errors.WrapIf(err, "error during getting Alertmanager ingress options")
		}
		result.Ingress.baseIngressSpec = *ingressSpec

		if ingressSpec.Enabled {
			result.Ingress.SecretId, err = askSecret(banzaiCLI, htPasswordSecretType, defaults.Ingress.SecretId, true)
			if err != nil {
				return nil, errors.WrapIf(err, "error during getting secret for Alertmanager ingress")
			}
		}
	}

	return result, nil
}

func askSecret(banzaiCLI cli.Cli, secretType, DefaultValue string, withSkipOption bool) (string, error) {

	orgID := banzaiCLI.Context().OrganizationID()
	secrets, _, err := banzaiCLI.Client().SecretsApi.GetSecrets(
		context.Background(),
		orgID,
		&pipeline.GetSecretsOpts{
			Type_: optional.NewString(secretType),
		},
	)
	if err != nil {
		return "", errors.WrapIfWithDetails(err, "failed to get secret(s)", "secretType", secretType)
	}

	if len(secrets) == 0 {
		// TODO (colin): add option to create new secret
		return "", nil
	}

	const skip = "skip"

	var secretName string
	var defaultSecretName string
	var secretLen = len(secrets)
	var secretIds = make(map[string]string, secretLen)
	if withSkipOption {
		defaultSecretName = skip
		secretLen = secretLen + 1
	}
	secretOptions := make([]string, secretLen)
	if withSkipOption {
		secretOptions[0] = skip
	}
	for i, s := range secrets {
		var idx = i
		if withSkipOption {
			idx = idx + 1
		}
		secretOptions[idx] = s.Name
		secretIds[s.Name] = s.Id
		if s.Id == DefaultValue {
			defaultSecretName = s.Name
		}
	}

	if err := input.DoQuestions([]input.QuestionMaker{input.QuestionSelect{
		QuestionInput: input.QuestionInput{
			QuestionBase: input.QuestionBase{
				Message: "Provider secret:",
			},
			DefaultValue: defaultSecretName,
			Output:       &secretName,
		},
		Options: secretOptions,
	}}); err != nil {
		return "", errors.WrapIf(err, "error during getting secret")
	}

	if secretName == skip {
		return "", nil
	}

	return secretIds[secretName], nil
}

func askNotificationProviderSlack(banzaiCLI cli.Cli, defaultsInterface interface{}) (*slackSpec, error) {
	var defaults slackSpec
	if err := mapstructure.Decode(defaultsInterface, &defaults); err != nil {
		return nil, errors.WrapIf(err, "failed to bind Slack config")
	}

	var err error
	var result = &slackSpec{
		Enabled: true,
	}
	result.SecretId, err = askSecret(banzaiCLI, slackSecretType, defaults.SecretId, false)
	if err != nil {
		return nil, errors.WrapIf(err, "error during getting Slack secret")
	}

	if err := input.DoQuestions([]input.QuestionMaker{
		input.QuestionInput{
			QuestionBase: input.QuestionBase{
				Message: "Provide Slack channel name for the alerts:",
			},
			Output: &result.Channel,
		},
		input.QuestionConfirm{
			QuestionBase: input.QuestionBase{
				Message: "Send resolved notifications as well",
			},
			DefaultValue: defaults.SendResolved,
			Output:       &result.SendResolved,
		},
	}); err != nil {
		return nil, errors.WrapIf(err, "error during getting Slack options")
	}

	return result, nil
}

func askNotificationProviderPagerDuty(banzaiCLI cli.Cli, defaultsInterface interface{}) (*pagerDutySpec, error) {
	var defaults pagerDutySpec
	if err := mapstructure.Decode(defaultsInterface, &defaults); err != nil {
		return nil, errors.WrapIf(err, "failed to bind PagerDuty config")
	}

	var result = &pagerDutySpec{
		Enabled: true,
	}

	// ask for pd URL
	if err := input.DoQuestions([]input.QuestionMaker{
		input.QuestionInput{
			QuestionBase: input.QuestionBase{
				Message: "Provide PagerDuty service endpoint:",
			},
			DefaultValue: defaults.Url,
			Output:       &result.Url,
		},
	}); err != nil {
		return nil, errors.WrapIf(err, "error during getting PagerDuty url")
	}

	// ask for pd integration type
	var integrationType string
	var defaultIntegrationValue = pdIntegrationTypePrometheusName
	if defaults.IntegrationType == pdIntegrationTypeEventsApiV2 {
		defaultIntegrationValue = pdIntegrationTypeEventsApiV2Name
	}

	if err := input.DoQuestions([]input.QuestionMaker{
		input.QuestionSelect{
			QuestionInput: input.QuestionInput{
				QuestionBase: input.QuestionBase{
					Message: "Select PagerDuty integration type:",
				},
				DefaultValue: defaultIntegrationValue,
				Output:       &integrationType,
			},
			Options: []string{pdIntegrationTypePrometheusName, pdIntegrationTypeEventsApiV2Name},
		},
	}); err != nil {
		return nil, errors.WrapIf(err, "error during getting PagerDuty integration type")
	}

	switch integrationType {
	case pdIntegrationTypePrometheusName:
		result.IntegrationType = pdIntegrationTypePrometheus
	case pdIntegrationTypeEventsApiV2Name:
		result.IntegrationType = pdIntegrationTypeEventsApiV2
	default:
		return nil, errors.NewWithDetails("invalid integration type", "type", integrationType)
	}

	// ask for pd secret
	var err error
	result.SecretId, err = askSecret(banzaiCLI, pagerDutySecretType, defaults.SecretId, false)
	if err != nil {
		return nil, errors.WrapIf(err, "error during getting PagerDuty secret")
	}

	if err := input.DoQuestions([]input.QuestionMaker{
		input.QuestionConfirm{
			QuestionBase: input.QuestionBase{
				Message: "Send resolved notifications as well",
			},
			DefaultValue: defaults.SendResolved,
			Output:       &result.SendResolved,
		},
	}); err != nil {
		return nil, errors.WrapIf(err, "error during getting PagerDuty send resolved option")
	}

	return result, nil
}

func askPushgateway(banzaiCLI cli.Cli, defaults pushgatewaySpec) (*pushgatewaySpec, error) {
	var isEnabled bool
	if err := input.DoQuestions([]input.QuestionMaker{
		input.QuestionConfirm{
			QuestionBase: input.QuestionBase{
				Message: "Do you want to enable Pushgateway?",
			},
			DefaultValue: defaults.Enabled,
			Output:       &isEnabled,
		},
	}); err != nil {
		return nil, errors.WrapIf(err, "error during getting Pushgateway enabled")
	}

	var result = &pushgatewaySpec{
		Enabled: isEnabled,
	}

	if isEnabled {
		// ask ingress
		ingressSpec, err := askIngress("Pushgateway", defaults.Ingress.baseIngressSpec)
		if err != nil {
			return nil, errors.WrapIf(err, "error during getting Pushgateway ingress options")
		}
		result.Ingress.baseIngressSpec = *ingressSpec

		if ingressSpec.Enabled {
			result.Ingress.SecretId, err = askSecret(banzaiCLI, htPasswordSecretType, defaults.Ingress.SecretId, true)
			if err != nil {
				return nil, errors.WrapIf(err, "error during getting secret for Pushgateway ingress")
			}
		}
	}

	return result, nil
}
