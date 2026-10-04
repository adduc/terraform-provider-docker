package internal

import (
	"context"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/moby/moby/client"
)

type FileDataSource struct {
	DockerClient *client.Client
}

type FileDataSourceModel struct {
	Container types.String `tfsdk:"container"`
	Path      types.String `tfsdk:"path"`
	File      types.Object `tfsdk:"file"`
	Stat      types.Object `tfsdk:"stat"`
}

func NewFileDataSource() datasource.DataSource {
	return &FileDataSource{}
}

func (d *FileDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_file"
}

func (d *FileDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve a single file's stats and contents from a docker container.\n\n" +
			"Use the `docker_files` data source to retrieve multiple files.",
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

			"file": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "The file at the path",
				Attributes:  fileSchemaAttributes(),
			},

			"stat": statSchemaAttribute(),
		},
	}
}

func (d *FileDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *FileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FileDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	stat, allFiles, diags := readContainerPath(ctx, d.DockerClient, data.Container.ValueString(), data.Path.ValueString())
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	if len(allFiles) == 0 {
		resp.Diagnostics.AddError(
			"No Files Found in Tar",
			fmt.Sprintf("No files were found in tar stream for %q", data.Path.ValueString()),
		)
		return
	}

	if len(allFiles) > 1 {
		fileNames := make([]string, 0, len(allFiles))
		for name := range allFiles {
			fileNames = append(fileNames, name)
		}
		slices.Sort(fileNames)
		resp.Diagnostics.AddError(
			"Multiple Files Found in Tar",
			fmt.Sprintf("Expected exactly one file in tar stream for %q, but found %d files: %v. Use the docker_files data source to read a directory.",
				data.Path.ValueString(), len(allFiles), fileNames),
		)
		return
	}

	for _, fileInfo := range allFiles {
		data.File = fileObject(fileInfo)
	}

	data.Stat = statObject(stat)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
