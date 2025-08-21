data "docker_files" "example" {
  container = "alpine"
  path      = "/etc/apk"
}
