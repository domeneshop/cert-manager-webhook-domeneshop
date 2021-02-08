package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/klog"

	//"k8s.io/client-go/kubernetes"

	"github.com/domeneshop/cert-manager-webhook-domeneshop/pkg/domeneshop"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
	"github.com/jetstack/cert-manager/pkg/issuer/acme/dns/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var groupName = os.Getenv("GROUP_NAME")

func main() {
	if groupName == "" {
		panic("GROUP_NAME must be specified")
	}

	versionInfo := domeneshop.GetVersion()
	klog.Infof("Initializing %s v%s (%s), built %s", "cert-manager-webhook-domeneshop", versionInfo.Version, versionInfo.GitCommit, versionInfo.BuildDate)

	// This will register our custom DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(groupName,
		&domeneshopDNSProviderSolver{},
	)
}

// domeneshopDNSProviderSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/jetstack/cert-manager/pkg/acme/webhook.Solver`
// interface.
type domeneshopDNSProviderSolver struct {
	// If a Kubernetes 'clientset' is needed, you must:
	// 1. uncomment the additional `client` field in this structure below
	// 2. uncomment the "k8s.io/client-go/kubernetes" import at the top of the file
	// 3. uncomment the relevant code in the Initialize method below
	// 4. ensure your webhook's service account has the required RBAC role
	//    assigned to it for interacting with the Kubernetes APIs you need.
	client *kubernetes.Clientset
}

// domeneshopDNSProviderConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
// This information is provided by cert-manager, and may be a reference to
// additional configuration that's needed to solve the challenge for this
// particular certificate or issuer.
// This typically includes references to Secret resources containing DNS
// provider credentials, in cases where a 'multi-tenant' DNS solver is being
// created.
// If you do *not* require per-issuer or per-certificate configuration to be
// provided to your webhook, you can skip decoding altogether in favour of
// using CLI flags or similar to provide configuration.
// You should not include sensitive information here. If credentials need to
// be used by your provider here, you should reference a Kubernetes Secret
// resource and fetch these credentials using a Kubernetes clientset.
type domeneshopDNSProviderConfig struct {
	// Change the two fields below according to the format of the configuration
	// to be decoded.
	// These fields will be set by users in the
	// `issuer.spec.acme.dns01.providers.webhook.config` field.

	//Email           string `json:"email"`
	APITokenSecretRef  corev1.SecretKeySelector `json:"apiTokenSecretRef"`
	APISecretSecretRef corev1.SecretKeySelector `json:"apiSecretSecretRef"`
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *domeneshopDNSProviderSolver) Name() string {
	return "domeneshop"
}

func (c *domeneshopDNSProviderSolver) getClient(ch *v1alpha1.ChallengeRequest) (*domeneshop.Client, error) {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return nil, err
	}

	apiToken, err := getSecretValue(c.client, ch.ResourceNamespace, cfg.APITokenSecretRef.Name, cfg.APITokenSecretRef.Key)
	if err != nil {
		return nil, err
	}
	apiSecret, err := getSecretValue(c.client, ch.ResourceNamespace, cfg.APISecretSecretRef.Name, cfg.APISecretSecretRef.Key)
	if err != nil {
		return nil, err
	}

	client := domeneshop.NewClient(apiToken, apiSecret)
	return client, nil
}

func getSecretValue(client *kubernetes.Clientset, namespace string, secretName string, secretKey string) (string, error) {

	secret, err := client.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	secretValue, ok := secret.Data[secretKey]
	if !ok {
		return "", fmt.Errorf("key %s not found in in secret '%s/%s'", secretKey, namespace, secretName)
	}
	return string(secretValue), nil
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *domeneshopDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {

	client, err := c.getClient(ch)
	if err != nil {
		return err
	}

	domain, err := client.GetDomainByName(util.UnFqdn(ch.ResolvedZone))
	if err != nil {
		return err
	}

	client.CreateTXTRecord(domain, util.UnFqdn(strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone)), ch.Key)

	return nil
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *domeneshopDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	client, err := c.getClient(ch)
	if err != nil {
		return err
	}

	domain, err := client.GetDomainByName(util.UnFqdn(ch.ResolvedZone))
	if err != nil {
		return err
	}

	if err := client.DeleteTXTRecord(domain, util.UnFqdn(strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone)), ch.Key); err != nil {
		return err
	}

	return nil
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *domeneshopDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {

	client, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}
	c.client = client

	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extapi.JSON) (domeneshopDNSProviderConfig, error) {
	cfg := domeneshopDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}
