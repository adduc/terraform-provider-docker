package internal

import (
	"archive/tar"
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
		MarkdownDescription: `
			Retrieve a single files stats and contents from a docker container.

			Use the docker_files data source to retrieve multiple files.
		`,
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

			"file": schema.ObjectAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The first file returned",
				AttributeTypes: map[string]attr.Type{
					"content":  types.StringType,
					"mod_time": types.StringType,
					"mode":     types.Int64Type,
					"name":     types.StringType,
				},
			},

			"stat": schema.ObjectAttribute{
				Computed:    true,
				Description: "Stat for file path",
				AttributeTypes: map[string]attr.Type{
					"name":        types.StringType,
					"size":        types.Int64Type,
					"mode":        types.Int32Type,
					"mtime":       types.StringType,
					"link_target": types.StringType,
				},
			},
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

	file, stat, err := d.DockerClient.CopyFromContainer(ctx, data.Container.ValueString(), data.Path.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read File from Container",
			fmt.Sprintf("Error reading file %q from container %q: %v", data.Path.ValueString(), data.Container.ValueString(), err),
		)
		return
	}
	defer file.Close()

	data.Stat = types.ObjectValueMust(
		map[string]attr.Type{
			"name":        types.StringType,
			"size":        types.Int64Type,
			"mode":        types.Int32Type,
			"mtime":       types.StringType,
			"link_target": types.StringType,
		},

		map[string]attr.Value{
			"name":        types.StringValue(stat.Name),
			"size":        types.Int64Value(stat.Size),
			"mode":        types.Int32Value(int32(stat.Mode)),
			"mtime":       types.StringValue(stat.Mtime.Format(time.RFC3339)),
			"link_target": types.StringValue(stat.LinkTarget),
		},
	)

	tr := tar.NewReader(file)
	header, content, err := extractFileFromTar(tr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Extract File from Tar",
			fmt.Sprintf("Error extracting file from tar stream for %q: %v", data.Path.ValueString(), err),
		)
		return
	}

	data.File = types.ObjectValueMust(
		map[string]attr.Type{
			"content":  types.StringType,
			"mod_time": types.StringType,
			"mode":     types.Int64Type,
			"name":     types.StringType,
		},

		map[string]attr.Value{
			"content":  types.StringValue(string(content)),
			"mod_time": types.StringValue(header.ModTime.Format(time.RFC3339)),
			"mode":     types.Int64Value(header.Mode),
			"name":     types.StringValue(header.Name),
		},
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// @todo error if multiple files are returned in tar
// @todo error if no files are returned in tar
