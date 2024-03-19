# Copyright 2022 Ross Light
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

terraform {
  required_version = "1.7.4"

  backend "remote" {
    organization = "zombiezen"

    workspaces {
      name = "tailscale-lb"
    }
  }

  required_providers {
    google = {
      version = "5.13.0"
    }
    github = {
      source  = "integrations/github"
      version = "5.43.0"
    }
  }
}

provider "google" {
  project = "zombiezen-tailscale-lb"
}

provider "github" {
  owner = local.github_repository_owner
}

data "google_project" "project" {
}

locals {
  github_repository_owner = "zombiezen"
  github_repository_name  = "tailscale-lb"
}

resource "google_project_service" "cloudresourcemanager" {
  service            = "cloudresourcemanager.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "iam" {
  service            = "iam.googleapis.com"
  disable_on_destroy = false
}

resource "google_service_account" "github_actions" {
  account_id   = "github"
  display_name = "GitHub Actions"
  description  = "GitHub Actions runners"
  depends_on = [
    google_project_service.iam,
  ]
}

module "nix_cache" {
  source = "zombiezen/nix-cache/google"
  version = "0.2.1"

  bucket_name = "${data.google_project.project.project_id}-nixcache"
  bucket_location      = "us"

  service_account_email = google_service_account.github_actions.email
  hmac_key = false
}

module "github_identity_pool" {
  source  = "zombiezen/github-identity/google"
  version = "0.1.2"

  attribute_condition = "assertion.repository=='${local.github_repository_owner}/${local.github_repository_name}'"

  service_accounts = {
    main = {
      subject              = "${local.github_repository_owner}/${local.github_repository_name}"
      service_account_name = google_service_account.github_actions.name
    }
  }
}

resource "github_actions_variable" "nix_substituter" {
  repository    = local.github_repository_name
  variable_name = "NIX_SUBSTITUTER"
  value         = module.nix_cache.nixcached_substituter
}

resource "github_actions_variable" "workload_identity_provider" {
  repository    = local.github_repository_name
  variable_name = "GOOGLE_WORKLOAD_IDENTITY_PROVIDER"
  value         = module.github_identity_pool.pool_provider_name
}

resource "github_actions_variable" "service_account" {
  repository    = local.github_repository_name
  variable_name = "GOOGLE_SERVICE_ACCOUNT"
  value         = google_service_account.github_actions.email
}
