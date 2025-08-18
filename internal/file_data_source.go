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
			Retrieve a single file's stats and contents from a docker container.

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

			"file": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "The first file returned",
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

	// Validate container name
	if err := validateContainerName(data.Container.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Invalid Container Name",
			fmt.Sprintf("Container name validation failed: %v", err),
		)
		return
	}

	// Validate and sanitize path
	sanitizedPath, err := sanitizePath(data.Path.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid File Path",
			fmt.Sprintf("Path validation failed for %q: %v", data.Path.ValueString(), err),
		)
		return
	}

	file, stat, err := d.DockerClient.CopyFromContainer(ctx, data.Container.ValueString(), sanitizedPath)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read File from Container",
			fmt.Sprintf("Error reading file %q from container %q: %v", data.Path.ValueString(), data.Container.ValueString(), err),
		)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			resp.Diagnostics.AddWarning(
				"Resource Cleanup Warning",
				fmt.Sprintf("Failed to close file stream for %q from container %q: %v", data.Path.ValueString(), data.Container.ValueString(), closeErr),
			)
		}
	}()

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

	if len(allFiles) == 0 {
		resp.Diagnostics.AddError(
			"No Files Found in Tar",
			fmt.Sprintf("No files were found in tar stream for %q", data.Path.ValueString()),
		)
		return
	}

	if len(allFiles) > 1 {
		var fileNames []string
		for name := range allFiles {
			fileNames = append(fileNames, name)
		}
		resp.Diagnostics.AddError(
			"Multiple Files Found in Tar",
			fmt.Sprintf("Expected exactly one file in tar stream for %q, but found %d files: %v", 
				data.Path.ValueString(), len(allFiles), fileNames),
		)
		return
	}

	// Get the single file
	var fileInfo *FileInfo
	for _, info := range allFiles {
		fileInfo = info
		break
	}

	data.File = types.ObjectValueMust(
		map[string]attr.Type{
			"content":  types.StringType,
			"gid":      types.Int32Type,
			"mod_time": types.StringType,
			"mode":     types.Int64Type,
			"name":     types.StringType,
			"size":     types.Int64Type,
			"uid":      types.Int32Type,
		},

		map[string]attr.Value{
			"content":  types.StringValue(string(fileInfo.Content)),
			"gid":      types.Int32Value(int32(fileInfo.Header.Gid)),
			"mod_time": types.StringValue(fileInfo.Header.ModTime.Format(time.RFC3339)),
			"mode":     types.Int64Value(fileInfo.Header.Mode),
			"name":     types.StringValue(fileInfo.Header.Name),
			"size":     types.Int64Value(fileInfo.Header.Size),
			"uid":      types.Int32Value(int32(fileInfo.Header.Uid)),
		},
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

