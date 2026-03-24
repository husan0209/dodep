fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .build_server(false)
        .build_client(true)
        .compile(
            &[
                "../../libs/proto/common/v1/types.proto",
                "../../libs/proto/betting/v1/betting.proto",
            ],
            &["../../libs/proto"],
        )?;
    Ok(())
}
