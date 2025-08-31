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

	// Deprecated fields that are still provided by the Docker API
	Version       types.String `tfsdk:"version"`
	APIVersion    types.String `tfsdk:"api_version"`
	MinAPIVersion types.String `tfsdk:"min_api_version"`
	GitCommit     types.String `tfsdk:"git_commit"`
	GoVersion     types.String `tfsdk:"go_version"`
	Os            types.String `tfsdk:"os"`
	Arch          types.String `tfsdk:"arch"`
	KernelVersion types.String `tfsdk:"kernel_version"`
	Experimental  types.Bool   `tfsdk:"experimental"`
	BuildTime     types.String `tfsdk:"build_time"`
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

			"version": schema.StringAttribute{
				Computed:    true,
				Description: "The Docker version",
			},
			"api_version": schema.StringAttribute{
				Computed:    true,
				Description: "The Docker API version",
			},
			"min_api_version": schema.StringAttribute{
				Computed:    true,
				Description: "The minimum Docker API version",
			},
			"git_commit": schema.StringAttribute{
				Computed:    true,
				Description: "The Git commit SHA",
			},
			"go_version": schema.StringAttribute{
				Computed:    true,
				Description: "The Go version",
			},
			"os": schema.StringAttribute{
				Computed:    true,
				Description: "The operating system",
			},
			"arch": schema.StringAttribute{
				Computed:    true,
				Description: "The architecture",
			},
			"kernel_version": schema.StringAttribute{
				Computed:    true,
				Description: "The kernel version",
			},
			"experimental": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether experimental features are enabled",
			},
			"build_time": schema.StringAttribute{
				Computed:    true,
				Description: "The build time",
			},

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

						// @todo implement details
						// Details map[string]string
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

	data.Version = types.StringValue(version.Version)
	data.APIVersion = types.StringValue(version.APIVersion)
	data.MinAPIVersion = types.StringValue(version.MinAPIVersion)
	data.GitCommit = types.StringValue(version.GitCommit)
	data.GoVersion = types.StringValue(version.GoVersion)
	data.Os = types.StringValue(version.Os)
	data.Arch = types.StringValue(version.Arch)
	data.KernelVersion = types.StringValue(version.KernelVersion)
	data.Experimental = types.BoolValue(version.Experimental)
	data.BuildTime = types.StringValue(version.BuildTime)

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
	}

	componentAttrs := make(map[string]attr.Value)
	for _, component := range version.Components {
		componentAttrs[component.Name] = types.ObjectValueMust(
			componentTypes,
			map[string]attr.Value{
				"name":    types.StringValue(component.Name),
				"version": types.StringValue(component.Version),
			},
		)
	}

	data.Components = types.MapValueMust(
		types.ObjectType{AttrTypes: componentTypes},
		componentAttrs,
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
