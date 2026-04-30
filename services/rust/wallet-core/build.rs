// Build script for wallet-core
// Generates Rust code from Protobuf definitions

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Provide protoc binary
    std::env::set_var("PROTOC", protoc_bin_vendored::protoc_bin_path().unwrap());

    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .compile(
            &[
                "../../../libs/proto/wallet/v1/wallet.proto",
                "../../../libs/proto/common/v1/types.proto",
                "../../../libs/proto/common/v1/money.proto",
                "../../../libs/proto/common/v1/error.proto",
            ],
            &["../../../libs/proto"],
        )?;

    Ok(())
}
