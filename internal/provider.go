package internal

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/adduc/terraform-provider-docker/internal/sshconn"
	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/moby/moby/client"
)

type Provider struct {
	version string
}

type ProviderModel struct {
	Host     types.String `tfsdk:"host"`
	CertPath types.String `tfsdk:"cert_path"`
	Timeout  types.Int32  `tfsdk:"timeout"`
}

type ProviderConfig struct {
	DockerClient *client.Client
}

func (p *Provider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "docker"
	resp.Version = p.version
}

func (p *Provider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read files, logs, and server information from Docker containers.\n\n" +
			"Connects to the local Docker daemon by default, or to a remote daemon over `tcp://` " +
			"(optionally with TLS) or `ssh://`. Like the Docker CLI, it reads `DOCKER_HOST`, " +
			"`DOCKER_CERT_PATH`, `DOCKER_TLS_VERIFY`, and `DOCKER_API_VERSION` from the environment.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Description: "The Docker daemon address, e.g. unix:///var/run/docker.sock, tcp://host:2376, or ssh://user@host. Defaults to DOCKER_HOST, then the local socket",
				Optional:    true,
			},
			"cert_path": schema.StringAttribute{
				Description: "Directory containing ca.pem, cert.pem, and key.pem for a TLS connection to the daemon. The server certificate is verified. Defaults to DOCKER_CERT_PATH (with DOCKER_TLS_VERIFY)",
				Optional:    true,
			},
			"timeout": schema.Int32Attribute{
				Description: "The timeout for Docker API requests, in seconds. Must be at least 1. Defaults to 30",
				Optional:    true,
				Validators: []validator.Int32{
					int32validator.AtLeast(1),
				},
			},
		},
	}
}

func (p *Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data ProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	opts, err := clientOpts(data)

	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Docker Host",
			"Failed to configure Docker host: "+err.Error(),
		)
		return
	}

	client, err := client.New(opts...)

	if err != nil {
		resp.Diagnostics.AddError(
			"Client Creation Failed",
			"Failed to create Docker client: "+err.Error(),
		)
		return
	}

	config := ProviderConfig{
		DockerClient: client,
	}

	resp.DataSourceData = config
	resp.ResourceData = config
}

// clientOpts returns the Docker client options for the provider
// configuration, falling back to the same environment variables as the
// Docker CLI: DOCKER_HOST, DOCKER_CERT_PATH, DOCKER_TLS_VERIFY, and
// DOCKER_API_VERSION.
func clientOpts(data ProviderModel) ([]client.Opt, error) {
	timeout := int32(30)
	if !data.Timeout.IsNull() && !data.Timeout.IsUnknown() {
		timeout = data.Timeout.ValueInt32()
	}

	// TLS is configured before the host, as client.FromEnv does.
	opts := []client.Opt{
		client.WithTLSClientConfigFromEnv(),
		client.WithAPIVersionFromEnv(),
		client.WithTimeout(time.Duration(timeout) * time.Second),
	}

	if certPath := data.CertPath.ValueString(); certPath != "" {
		opts = append(opts, client.WithTLSClientConfig(
			filepath.Join(certPath, "ca.pem"),
			filepath.Join(certPath, "cert.pem"),
			filepath.Join(certPath, "key.pem"),
		))
	}

	// DOCKER_HOST is resolved here rather than with client.WithHostFromEnv
	// so that ssh:// hosts from the environment use the ssh dialer too.
	host := data.Host.ValueString()
	if host == "" {
		host = os.Getenv(client.EnvOverrideHost)
	}

	if host != "" {
		extra, err := hostOpts(host)
		if err != nil {
			return nil, err
		}
		opts = append(opts, extra...)
	}

	return opts, nil
}

// hostOpts returns the client options for connecting to host. ssh:// hosts
// are dialed through the local ssh binary; anything else (tcp://, unix://,
// npipe://, ...) is handled by the Docker client directly.
func hostOpts(host string) ([]client.Opt, error) {
	u, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "ssh" {
		return []client.Opt{client.WithHost(host)}, nil
	}

	dial, err := sshconn.NewDialer(host)
	if err != nil {
		return nil, err
	}

	return []client.Opt{
		client.WithHost(sshconn.Host),
		client.WithDialContext(dial),
	}, nil
}

func (p *Provider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewFileDataSource,
		NewFilesDataSource,
		NewLogsDataSource,
		NewServerVersionDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &Provider{
			version: version,
		}
	}
}
