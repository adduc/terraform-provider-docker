data "docker_file" "example" {
  container = "alpine"
  path      = "/etc/apk/repositories"
}
