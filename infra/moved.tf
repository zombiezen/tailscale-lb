moved {
  from = google_storage_bucket.nixcache
  to = module.nix_cache.google_storage_bucket.cache
}

moved {
  from = google_project_service.storage
  to = module.nix_cache.google_project_service.storage
}
