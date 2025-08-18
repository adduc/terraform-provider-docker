package internal

import (
	"context"
	"time"

	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Provider struct {
	version string
}

type ProviderModel struct {
	Host    types.String `tfsdk:"host"`
	Timeout types.Int32  `tfsdk:"timeout"`
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
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Description: "The Docker daemon address",
				Optional:    true,
			},
			"timeout": schema.Int32Attribute{
				MarkdownDescription: `
					The timeout for Docker API requests

					Default: 30 seconds
				`,
				Optional: true,
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

	timeout := int32(30)
	if !data.Timeout.IsNull() && !data.Timeout.IsUnknown() {
		timeout = data.Timeout.ValueInt32()
	}

	opts := []client.Opt{
		client.WithTimeout(time.Duration(timeout) * time.Second),
		client.WithAPIVersionNegotiation(),
	}

	if data.Host.ValueString() != "" {
		helper, err := connhelper.GetConnectionHelper(data.Host.ValueString())

		if err != nil {
			resp.Diagnostics.AddError(
				"Connection Helper Error",
				"Failed to get connection helper: "+err.Error(),
			)
			return
		}

		opts = append(
			opts,
			client.WithHost(helper.Host),
			client.WithDialContext(helper.Dialer),
		)
	}

	client, err := client.NewClientWithOpts(opts...)

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

func (p *Provider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewFileDataSource,
		NewFilesDataSource,
		NewLogsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &Provider{
			version: version,
		}
	}
}
