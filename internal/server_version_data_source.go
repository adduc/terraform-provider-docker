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
	Platform   types.Object `tfsdk:"platform"`
	Components types.Map    `tfsdk:"components"`
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
		Retrieves Docker server version information, including platform details,
		version metadata, and component information.

		Use this data source to access details about the Docker server your
		provider is connected to.
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

			"components": schema.MapNestedAttribute{
				Computed:    true,
				Description: "Components information",

				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The component name",
						},
						"version": schema.StringAttribute{
							Computed:    true,
							Description: "The component version",
						},
						"details": schema.MapAttribute{
							Computed:    true,
							Description: "Additional details about the component",
							ElementType: types.StringType,
						},
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
			"Failed to retrieve Docker server version",
			err.Error(),
		)
		return
	}

	data.Platform = types.ObjectValueMust(
		map[string]attr.Type{
			"name": types.StringType,
		},
		map[string]attr.Value{
			"name": types.StringValue(version.Platform.Name),
		},
	)

	componentTypes := map[string]attr.Type{
		"name":    types.StringType,
		"version": types.StringType,
		"details": types.MapType{ElemType: types.StringType},
	}

	componentAttrs := make(map[string]attr.Value)
	for _, component := range version.Components {

		details := map[string]attr.Value{}
		for key, value := range component.Details {
			details[key] = types.StringValue(value)
		}

		componentAttrs[component.Name] = types.ObjectValueMust(
			componentTypes,
			map[string]attr.Value{
				"name":    types.StringValue(component.Name),
				"version": types.StringValue(component.Version),
				"details": types.MapValueMust(
					types.StringType,
					details,
				),
			},
		)
	}

	data.Components = types.MapValueMust(
		types.ObjectType{AttrTypes: componentTypes},
		componentAttrs,
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
