fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .build_server(false)
        .build_client(true)
        .compile(
            &[
                // build.rs runs with the crate directory as cwd, so reaching the
                // repository's libs/proto needs three levels up, not two. The
                // two-level form resolved outside the repo and protoc aborted
                // with "Could not make proto path relative". The proto set is
                // unchanged - only the paths were wrong.
                "../../../libs/proto/common/v1/types.proto",
                "../../../libs/proto/betting/v1/betting.proto",
            ],
            &["../../../libs/proto"],
        )?;
    Ok(())
}
