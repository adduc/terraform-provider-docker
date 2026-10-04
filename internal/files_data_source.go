package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/moby/moby/client"
)

type FilesDataSource struct {
	DockerClient *client.Client
}

type FilesDataSourceModel struct {
	Container types.String `tfsdk:"container"`
	Path      types.String `tfsdk:"path"`
	Files     types.Map    `tfsdk:"files"`
	Stat      types.Object `tfsdk:"stat"`
}

func NewFilesDataSource() datasource.DataSource {
	return &FilesDataSource{}
}

func (d *FilesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_files"
}

func (d *FilesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve files' stats and contents from a docker container.\n\n" +
			"Returns all files in the specified path as a map.",
		Attributes: map[string]schema.Attribute{

			// Required

			"container": schema.StringAttribute{
				Required:    true,
				Description: "The name of the container",
			},

			"path": schema.StringAttribute{
				Required:    true,
				Description: "The filepath to request from the container",
			},

			// Computed

			"files": schema.MapNestedAttribute{
				Computed:    true,
				Description: "All files returned from the path, keyed by their path within the archive",
				NestedObject: schema.NestedAttributeObject{
					Attributes: fileSchemaAttributes(),
				},
			},

			"stat": statSchemaAttribute(),
		},
	}
}

func (d *FilesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *FilesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FilesDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	stat, allFiles, diags := readContainerPath(ctx, d.DockerClient, data.Container.ValueString(), data.Path.ValueString())
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Stat = statObject(stat)

	fileAttrs := make(map[string]attr.Value, len(allFiles))
	for fileName, fileInfo := range allFiles {
		fileAttrs[fileName] = fileObject(fileInfo)
	}

	data.Files = types.MapValueMust(types.ObjectType{AttrTypes: fileAttrTypes}, fileAttrs)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
