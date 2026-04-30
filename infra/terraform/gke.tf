# GKE Cluster configuration

data "google_client_config" "default" {}

module "gke" {
  source  = "terraform-google-modules/kubernetes-engine/google"
  version = "~> 30.0"
  
  project_id                        = var.project_id
  name                              = var.cluster_name
  region                            = var.region
  zones                             = ["us-central1-a", "us-central1-b", "us-central1-c"]
  network                           = google_compute_network.vpc.name
  subnetwork                        = google_compute_subnetwork.subnet.name
  ip_range_pods                     = "opus-casino-pods"
  ip_range_services                 = "opus-casino-services"
  
  http_load_balancing               = false
  network_policy                    = false
  horizontal_pod_autoscaling        = true
  filestore_csi_driver              = false
  dns_cache                         = true
  deletion_protection               = false
  
  node_pools = [
    {
      name               = "system-pool"
      machine_type       = "n2-standard-2"
      min_count          = 3
      max_count          = 10
      disk_size_gb       = 50
      disk_type          = "pd-ssd"
      auto_repair        = true
      auto_upgrade       = true
      preemptible        = false
    },
    {
      name               = "rust-services-pool"
      machine_type       = var.machine_type
      min_count          = var.min_nodes
      max_count          = var.max_nodes
      disk_size_gb       = var.disk_size_gb
      disk_type          = "pd-ssd"
      auto_repair        = true
      auto_upgrade       = true
      preemptible        = false
    },
    {
      name               = "go-services-pool"
      machine_type       = "n2-standard-2"
      min_count          = 3
      max_count          = 15
      disk_size_gb       = 50
      disk_type          = "pd-ssd"
      auto_repair        = true
      auto_upgrade       = true
      preemptible        = false
    },
    {
      name               = "data-pool"
      machine_type       = "n2-highmem-4"
      min_count          = 3
      max_count          = 10
      disk_size_gb       = 200
      disk_type          = "pd-ssd"
      auto_repair        = true
      auto_upgrade       = true
      preemptible        = false
    },
  ]
  
  node_pools_oauth_scopes = {
    all = [
      "https://www.googleapis.com/auth/cloud-platform",
      "https://www.googleapis.com/auth/logging.write",
      "https://www.googleapis.com/auth/monitoring",
    ]
  }
  
  node_pools_labels = {
    all = {}
    
    default-node-pool = {
      default-node-pool = true
    }
  }
  
  node_pools_metadata = {
    all = {}
    
    default-node-pool = {
      disable-legacy-endpoints = "true"
    }
  }
  
  node_pools_taints = {
    all = []
    
    default-node-pool = []
  }
  
  registry_project_ids = [var.project_id]
}
