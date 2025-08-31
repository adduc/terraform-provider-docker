package internal

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ServerVersionDataSource struct {
	DockerClient *client.Client
}

type ServerVersionDataSourceModel struct {
	Platform types.Object `tfsdk:"platform"`
}

func NewServerVersionDataSource() datasource.DataSource {
	return &ServerVersionDataSource{}
}

func (d *ServerVersionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_version"
}

func (d *ServerVersionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
		Retrieve a single file's stats and contents from a docker container.

		Use the docker_files data source to retrieve multiple files.
		`,
		Attributes: map[string]schema.Attribute{

			// Computed

			"platform": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Platform information",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Computed:    true,
						Description: "The platform name",
					},
				},
			},
		},
	}
}

func (d *ServerVersionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(ProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.DockerClient = config.DockerClient
}

func (d *ServerVersionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ServerVersionDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	version, err := d.DockerClient.ServerVersion(ctx)

	if err != nil {
		resp.Diagnostics.AddError(
			"Server Version query failed",
			err.Error(),
		)
	}

	data.Platform = types.ObjectValueMust(
		map[string]attr.Type{
			"name": types.StringType,
		},
		map[string]attr.Value{
			"name": types.StringValue(version.Os),
		},
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
