// Build script for wallet-core
// Generates Rust code from Protobuf definitions

use std::env;
use std::path::PathBuf;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let out_dir = PathBuf::from(env::var("OUT_DIR").unwrap());
    
    // Proto files location
    let proto_dir = PathBuf::from("../../libs/proto");
    
    // Proto files to compile
    let proto_files = vec![
        proto_dir.join("wallet/v1/wallet.proto"),
        proto_dir.join("common/v1/types.proto"),
        proto_dir.join("common/v1/money.proto"),
        proto_dir.join("common/v1/error.proto"),
    ];
    
    // Include paths
    let includes = vec![proto_dir.clone()];
    
    // Configure prost
    let mut config = prost_build::Config::new();
    config
        .out_dir(&out_dir)
        .enable_type_names()
        .type_name_domain(&["wallet.v1", "common.v1"], "wallet_core");
    
    // Generate gRPC code
    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .out_dir(&out_dir)
        .compile_with_config(config, &proto_files, &includes)?;
    
    println!("cargo:rerun-if-changed={}", proto_dir.display());
    
    Ok(())
}
