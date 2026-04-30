fn main() -> Result<(), Box<dyn std::error::Error>> {
    std::env::set_var("PROTOC", protoc_bin_vendored::protoc_bin_path().unwrap());
    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .compile(
            &[
                "../../../libs/proto/types.proto",
                "../../../libs/proto/betting.proto",
                "../../../libs/proto/wallet.proto",
            ],
            &["../../../libs/proto"],
        )?;
    Ok(())
}
