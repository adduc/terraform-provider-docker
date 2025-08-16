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
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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
		MarkdownDescription: `
			Retrieve files' stats and contents from a docker container.

			Returns all files in the specified path as a map.
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

			"files": schema.MapNestedAttribute{
				Computed:    true,
				Description: "All files returned from the path",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"content": schema.StringAttribute{
							Computed:    true,
							Sensitive:   true,
							Description: "The file content",
						},
						"mod_time": schema.StringAttribute{
							Computed:    true,
							Description: "The file modification time",
						},
						"mode": schema.Int64Attribute{
							Computed:    true,
							Description: "The file mode",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The file name",
						},
						"size": schema.Int64Attribute{
							Computed:    true,
							Description: "The file size",
						},
						"uid": schema.Int32Attribute{
							Computed:    true,
							Description: "The file owner UID",
						},
						"gid": schema.Int32Attribute{
							Computed:    true,
							Description: "The file owner GID",
						},
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "The file type",
						},
					},
				},
			},

			"stat": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Stat for file path",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Computed:    true,
						Description: "The file name",
					},
					"size": schema.Int64Attribute{
						Computed:    true,
						Description: "The file size",
					},
					"mode": schema.Int32Attribute{
						Computed:    true,
						Description: "The file mode",
					},
					"mtime": schema.StringAttribute{
						Computed:    true,
						Description: "The file modification time",
					},
					"link_target": schema.StringAttribute{
						Computed:    true,
						Description: "The file link target",
					},
				},
			},
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
	allFiles, err := extractAllFilesFromTar(tr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Extract Files from Tar",
			fmt.Sprintf("Error extracting files from tar stream for %q: %v", data.Path.ValueString(), err),
		)
		return
	}

	attrTypes := map[string]attr.Type{
		"content":  types.StringType,
		"gid":      types.Int32Type,
		"mod_time": types.StringType,
		"mode":     types.Int64Type,
		"name":     types.StringType,
		"size":     types.Int64Type,
		"uid":      types.Int32Type,
		"type":     types.StringType,
	}

	fileAttrs := make(map[string]attr.Value)
	for fileName, fileInfo := range allFiles {

		var content basetypes.StringValue
		if fileInfo.Content == nil {
			content = basetypes.NewStringNull()
		} else {
			content = types.StringValue(string(fileInfo.Content))
		}

		fileAttrs[fileName] = types.ObjectValueMust(
			attrTypes,
			map[string]attr.Value{
				"content":  content,
				"gid":      types.Int32Value(int32(fileInfo.Header.Gid)),
				"mod_time": types.StringValue(fileInfo.Header.ModTime.Format(time.RFC3339)),
				"mode":     types.Int64Value(fileInfo.Header.Mode),
				"name":     types.StringValue(fileInfo.Header.Name),
				"size":     types.Int64Value(fileInfo.Header.Size),
				"uid":      types.Int32Value(int32(fileInfo.Header.Uid)),
				"type":     types.StringValue(string(fileInfo.Header.Typeflag)),
			},
		)
	}

	data.Files = types.MapValueMust(
		types.ObjectType{AttrTypes: attrTypes},
		fileAttrs,
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
