fn main() -> Result<(), Box<dyn std::error::Error>> {
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
