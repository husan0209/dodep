fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Ship protoc with the build instead of requiring it on the machine:
    // prost-build aborts with "Could not find `protoc`" on CI, where the
    // runner image has no protobuf-compiler installed. wallet-core already
    // does this, which keeps the two crates consistent and hermetic.
    if std::env::var_os("PROTOC").is_none() {
        if let Ok(path) = protoc_bin_vendored::protoc_bin_path() {
            std::env::set_var("PROTOC", path);
        }
    }

    tonic_build::configure()
        .build_server(false)
        .build_client(true)
        .compile(
            &[
                // Three levels up: websocket-gateway -> rust -> services -> repo
                // root. Two levels resolves to services/libs, which does not
                // exist, so protoc aborted with "Could not make proto path
                // relative" before it was ever able to complain about anything
                // else.
                "../../../libs/proto/common/v1/types.proto",
                "../../../libs/proto/betting/v1/betting.proto",
            ],
            &["../../../libs/proto"],
        )?;
    Ok(())
}
