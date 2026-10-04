terraform {
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 4.6"
    }

    adduc-docker = {
      source  = "adduc/docker"
      version = "~> 0.0.5"
    }

    kubectl = {
      source  = "alekc/kubectl"
      version = "~> 2.4"
    }
  }
}

provider "docker" {}

provider "adduc-docker" {}

locals {
  kubeconfig = yamldecode(base64decode(data.docker_file.kubeconfig.file.content_base64))
}

provider "kubectl" {
  host                   = "https://127.0.0.1:${docker_container.k3s.ports[0].external}"
  cluster_ca_certificate = base64decode(local.kubeconfig.clusters[0].cluster["certificate-authority-data"])
  client_certificate     = base64decode(local.kubeconfig.users[0].user["client-certificate-data"])
  client_key             = base64decode(local.kubeconfig.users[0].user["client-key-data"])
  load_config_file       = false
}

resource "docker_image" "k3s" {
  name = "rancher/k3s:v1.32.1-k3s1"
}

resource "docker_container" "k3s" {
  name       = "k3s"
  image      = docker_image.k3s.image_id
  privileged = true

  entrypoint = ["sh", "-c"]
  command = [join(" && ", [
    # Avoids "path /var/lib/kubelet/pods is mounted on /var/lib/kubelet
    # but it is not a shared mount".
    # https://github.com/k3d-io/k3d/issues/1063#issuecomment-1153271637
    "mount --make-rshared /",
    "exec k3s server --disable=traefik,servicelb,metrics-server --node-name=k3s",
  ])]

  tmpfs = {
    "/run"     = ""
    "/var/run" = ""
  }

  ports {
    internal = 6443
    external = 6443
  }

  # Wait for the API server, so the kubeconfig exists before it is read.
  wait = true
  healthcheck {
    test     = ["CMD-SHELL", "kubectl get nodes"]
    interval = "2s"
    timeout  = "2s"
    retries  = 30
  }
}

data "docker_file" "kubeconfig" {
  provider  = adduc-docker
  container = docker_container.k3s.id
  path      = "/etc/rancher/k3s/k3s.yaml"
}

resource "kubectl_manifest" "nginx" {
  yaml_body = yamlencode({
    apiVersion = "apps/v1"
    kind       = "Deployment"
    metadata   = { name = "nginx", namespace = "default" }
    spec = {
      replicas = 1
      selector = { matchLabels = { app = "nginx" } }
      template = {
        metadata = { labels = { app = "nginx" } }
        spec = {
          containers = [{ name = "nginx", image = "nginx:1.29" }]
        }
      }
    }
  })
}
