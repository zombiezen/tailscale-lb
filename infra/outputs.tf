output "cache_bucket" {
  value = "gs://${google_storage_bucket.nixcache.name}"
}
