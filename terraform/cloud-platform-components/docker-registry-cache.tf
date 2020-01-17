resource "kubernetes_namespace" "docker-registry-cache" {
  metadata {
    name = "docker-registry-cache"

    labels = {
      "name"                                           = "docker-registry-cache"
      "component"                                      = "docker-registry"
      "cloud-platform.justice.gov.uk/environment-name" = "production"
      "cloud-platform.justice.gov.uk/is-production"    = "true"
    }

    annotations = {
      "cloud-platform.justice.gov.uk/application"   = "docker-registry-cache"
      "cloud-platform.justice.gov.uk/business-unit" = "cloud-platform"
      "cloud-platform.justice.gov.uk/owner"         = "Cloud Platform: platforms@digital.justice.gov.uk"
      "cloud-platform.justice.gov.uk/source-code"   = "https://github.com/ministryofjustice/cloud-platform-docker-registry-cache"
    }
  }
}

resource "kubernetes_limit_range" "docker-registry-cache" {
  metadata {
    name      = "limitrange"
    namespace = kubernetes_namespace.docker-registry-cache.id
  }

  spec {
    limit {
      type = "Container"
      default = {
        cpu    = "1"
        memory = "1000Mi"
      }
      default_request = {
        cpu    = "10m"
        memory = "100Mi"
      }
    }
  }
}

resource "kubernetes_resource_quota" "docker-registry-cache" {
  metadata {
    name      = "namespace-quota"
    namespace = kubernetes_namespace.docker-registry-cache.id
  }
  spec {
    hard = {
      pods = 50
    }
  }
}


resource "kubernetes_network_policy" "docker-registry-cache_default" {
  metadata {
    name      = "default"
    namespace = kubernetes_namespace.docker-registry-cache.id
  }

  spec {
    pod_selector {}

    ingress {
      from {
        pod_selector {}
      }
    }

    policy_types = ["Ingress"]
  }
}

resource "kubernetes_network_policy" "docker-registry-cache_allow_ingress_controllers" {
  metadata {
    name      = "allow-ingress-controllers"
    namespace = kubernetes_namespace.docker-registry-cache.id
  }

  spec {
    pod_selector {}

    ingress {
      from {
        namespace_selector {
          match_labels = {
            component = "ingress-controllers"
          }
        }
      }
    }

    policy_types = ["Ingress"]
  }
}


resource "null_resource" "docker-registry-cache-namespace-config" {
  provisioner "local-exec" {
    command = "kubectl apply -n docker-registry-cache -f ${path.module}/templates/docker-registry-cache/namespace.yaml"
  }

  provisioner "local-exec" {
    when    = destroy
    command = "exit 0"
  }

  triggers = {
    namespace = filesha1("${path.module}/templates/docker-registry-cache/namespace.yaml")
  }

  depends_on = [kubernetes_namespace.docker-registry-cache]
}

resource "null_resource" "docker-registry-cache" {
  # Everything in the namespace will be destroyed when the namespace is deleted.
  # We need the `exit 0` here so that terraform thinks it has successfully
  # destroyed the resource, when it applies changes (by destroying then applying)
  provisioner "local-exec" {
    when    = destroy
    command = "exit 0"
  }

  triggers = {
    contents = filesha1("${path.module}/templates/docker-registry-cache/docker-registry-cache.yaml.tpl")
  }

  depends_on = [kubernetes_namespace.docker-registry-cache]

  provisioner "local-exec" {
    command = <<EOS
kubectl apply -n docker-registry-cache -f - <<EOF
${
    templatefile("./templates/docker-registry-cache/docker-registry-cache.yaml.tpl", {
      cluster_name    = terraform.workspace,
      nat_gateway_ips = data.terraform_remote_state.network.outputs.nat_gateway_ips,
    })
  }
EOF
EOS
}

}
