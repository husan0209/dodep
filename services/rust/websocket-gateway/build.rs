// Build script for websocket-gateway
// Generates Rust code from Protobuf definitions.
// Uses the vendored protoc so no system protobuf installation is required.

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Provide protoc binary
    std::env::set_var("PROTOC", protoc_bin_vendored::protoc_bin_path().unwrap());

    tonic_build::configure()
        .build_server(false)
        .build_client(true)
        .compile(
            &[
                "../../../libs/proto/common/v1/types.proto",
                "../../../libs/proto/betting/v1/betting.proto",
            ],
            &["../../../libs/proto"],
        )?;

    Ok(())
}
