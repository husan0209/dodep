fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .compile(
            &[
                "../../libs/proto/types.proto",
                "../../libs/proto/betting.proto",
                "../../libs/proto/wallet.proto",
            ],
            &["../../libs/proto"],
        )?;
    Ok(())
}
