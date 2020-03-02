package k8s

import (
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	// The following line loads the gcp plugin which is required to authenticate against GKE clusters.
	// See: https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/gruntwork-io/terratest/modules/logger"
)

// GetKubernetesClientE returns a Kubernetes API client that can be used to make requests.
func GetKubernetesClientE(t *testing.T) (*kubernetes.Clientset, error) {
	kubeConfigPath, err := GetKubeConfigPathE(t)
	if err != nil {
		return nil, err
	}

	options := NewKubectlOptions("", kubeConfigPath, "default")
	return GetKubernetesClientFromOptionsE(t, options)
}

// GetKubernetesClientFromOptionsE returns a Kubernetes API client given a configured KubectlOptions object.
func GetKubernetesClientFromOptionsE(t *testing.T, options *KubectlOptions) (*kubernetes.Clientset, error) {
	var err error

	kubeConfigPath, err := options.GetConfigPath(t)
	if err != nil {
		return nil, err
	}
	logger.Logf(t, "Configuring kubectl using config file %s with context %s", kubeConfigPath, options.ContextName)
	// Load API config (instead of more low level ClientConfig)
	config, err := LoadApiClientConfigE(kubeConfigPath, options.ContextName)
	if err != nil {
		return nil, err
	}

	if options.Env["https_proxy"] != "" {
		// TODO: Consider using golang's no_proxy support to test for exclusion to kube API server.
		// TODO: This is making a new transport every time we get a new kubernetes client. Not ideal.
		proxyURL, err := url.Parse(options.Env["https_proxy"])
		if err != nil {
			return nil, err
		}
		logger.Logf(t, "Using proxy: %v", proxyURL)

		// Overwrite TLS-related fields from config to avoid collision with
		// Transport field. For more information, see:
		// * https://github.com/kubernetes/client-go/issues/452
		//

		transportConfig, err := config.TransportConfig()
		if err != nil {
			return nil, err
		}
		tlsConfig, err := transport.TLSConfigFor(transportConfig)
		if err != nil {
			return nil, err
		}
		config.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: tlsConfig,
			TLSHandshakeTimeout: 10 * time.Second,
			MaxIdleConnsPerHost: 100,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		}
		config.WrapTransport = transportConfig.WrapTransport
		config.Dial	= transportConfig.Dial
		config.TLSClientConfig = restclient.TLSClientConfig{}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
