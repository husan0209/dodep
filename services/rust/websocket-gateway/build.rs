fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Nothing in this crate calls `tonic::include_proto!`, so this build script
    // currently generates gRPC clients that are never linked in. It is kept --
    // removing it would also mean dropping the `tonic-build` dependency -- but
    // the proto paths were wrong: `../../libs/proto` resolves to `services/libs/
    // proto`, which does not exist, so protoc aborted with
    //
    //     error: protoc failed: Could not make proto path relative
    //
    // The contract lives at the repository root (CONVENTIONS.md NEVER-3), three
    // levels up from the crate, which is what the other two crates already use.
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
