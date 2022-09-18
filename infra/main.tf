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
  required_version = "1.2.9"

  backend "remote" {
    organization = "zombiezen"

    workspaces {
      name = "tailscale-lb"
    }
  }

  required_providers {
    google = {
      version = "4.34.0"
    }
    github = {
      source  = "integrations/github"
      version = "4.31.0"
    }
  }
}

provider "google" {
  project = "zombiezen-tailscale-lb"
}

provider "github" {
  owner = "zombiezen"
}

data "google_project" "project" {
}

resource "google_project_service" "cloudresourcemanager" {
  service            = "cloudresourcemanager.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "storage" {
  service            = "storage.googleapis.com"
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

resource "google_storage_hmac_key" "github_actions" {
  service_account_email = google_service_account.github_actions.email
}

resource "github_actions_secret" "gcs_hmac_access_id" {
  repository      = "tailscale-lb"
  secret_name     = "GCS_HMAC_ACCESS_ID"
  plaintext_value = google_storage_hmac_key.github_actions.access_id
}

resource "github_actions_secret" "gcs_hmac_secret" {
  repository      = "tailscale-lb"
  secret_name     = "GCS_HMAC_SECRET_ACCESS_KEY"
  plaintext_value = google_storage_hmac_key.github_actions.secret
}

resource "google_storage_bucket" "nixcache" {
  name          = "${data.google_project.project.project_id}-nixcache"
  location      = "us"
  storage_class = "STANDARD"

  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      num_newer_versions = 2
      with_state         = "ARCHIVED"
    }
  }

  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      days_since_noncurrent_time = 7
    }
  }

  depends_on = [
    google_project_service.storage,
  ]
}

resource "google_storage_bucket_iam_member" "github_nixcache_viewer" {
  bucket = google_storage_bucket.nixcache.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_storage_bucket_iam_member" "github_nixcache_creator" {
  bucket = google_storage_bucket.nixcache.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${google_service_account.github_actions.email}"
}
