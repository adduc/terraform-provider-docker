package internal

import (
	"archive/tar"
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// Shared by docker_file and docker_files, which both read a path from a
// container through the archive API.

// readContainerPath copies path out of a container, returning the path's
// stat and the entries of the resulting archive keyed by name.
func readContainerPath(ctx context.Context, c *client.Client, containerName, path string) (stat container.PathStat, files map[string]*FileInfo, diags diag.Diagnostics) {
	res, err := c.CopyFromContainer(ctx, containerName, client.CopyFromContainerOptions{SourcePath: path})
	if err != nil {
		diags.AddError(
			"Unable to Read File from Container",
			fmt.Sprintf("Error reading file %q from container %q: %v", path, containerName, err),
		)
		return stat, nil, diags
	}
	defer func() {
		if closeErr := res.Content.Close(); closeErr != nil {
			diags.AddWarning(
				"Resource Cleanup Warning",
				fmt.Sprintf("Failed to close file stream for %q from container %q: %v", path, containerName, closeErr),
			)
		}
	}()

	files, err = extractAllFilesFromTar(tar.NewReader(res.Content))
	if err != nil {
		diags.AddError(
			"Unable to Extract Files from Tar",
			fmt.Sprintf("Error extracting files from tar stream for %q: %v", path, err),
		)
		return res.Stat, nil, diags
	}

	return res.Stat, files, diags
}

var statAttrTypes = map[string]attr.Type{
	"name":        types.StringType,
	"size":        types.Int64Type,
	"mode":        types.Int64Type,
	"mtime":       types.StringType,
	"link_target": types.StringType,
}

func statSchemaAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
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
			"mode": schema.Int64Attribute{
				Computed:    true,
				Description: "The file mode, as Go os.FileMode bits reported by the Docker API",
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
	}
}

func statObject(stat container.PathStat) types.Object {
	return types.ObjectValueMust(statAttrTypes, map[string]attr.Value{
		"name":        types.StringValue(stat.Name),
		"size":        types.Int64Value(stat.Size),
		"mode":        types.Int64Value(int64(stat.Mode)),
		"mtime":       types.StringValue(stat.Mtime.Format(time.RFC3339)),
		"link_target": types.StringValue(stat.LinkTarget),
	})
}

var fileAttrTypes = map[string]attr.Type{
	"content_base64": types.StringType,
	"gid":            types.Int32Type,
	"mod_time":       types.StringType,
	"mode":           types.Int64Type,
	"name":           types.StringType,
	"size":           types.Int64Type,
	"uid":            types.Int32Type,
	"type":           types.StringType,
}

func fileSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"content_base64": schema.StringAttribute{
			Computed:    true,
			Sensitive:   true,
			Description: "The file content, base64-encoded. Null if it is not a regular file",
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
			Description: "The file type, as a tar type flag (\"0\" for a regular file, \"5\" for a directory, \"2\" for a symlink)",
		},
	}
}

func fileObject(info *FileInfo) types.Object {
	return types.ObjectValueMust(fileAttrTypes, map[string]attr.Value{
		"content_base64": fileContentBase64(info),
		"gid":            types.Int32Value(int32(info.Header.Gid)),
		"mod_time":       types.StringValue(info.Header.ModTime.Format(time.RFC3339)),
		"mode":           types.Int64Value(info.Header.Mode),
		"name":           types.StringValue(info.Header.Name),
		"size":           types.Int64Value(info.Header.Size),
		"uid":            types.Int32Value(int32(info.Header.Uid)),
		"type":           types.StringValue(string(info.Header.Typeflag)),
	})
}
