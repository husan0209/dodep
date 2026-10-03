fn main() -> Result<(), Box<dyn std::error::Error>> {
    std::env::set_var("PROTOC", protoc_bin_vendored::protoc_bin_path().unwrap());
    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .compile(
            &[
                // The protos live at libs/proto/<service>/v1/<service>.proto.
                // The flat names this used to list (libs/proto/types.proto,
                // libs/proto/betting.proto, ...) were moved into that tree, so
                // protoc aborted with
                //   Could not make proto path relative: ../../../libs/proto/types.proto
                // and no Rust service compiled at all.
                "../../../libs/proto/betting/v1/betting.proto",
                "../../../libs/proto/wallet/v1/wallet.proto",
            ],
            &["../../../libs/proto"],
        )?;
    Ok(())
}
