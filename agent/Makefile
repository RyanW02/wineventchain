run:
	export RUST_LOG=warn,agent=info
	cargo run

build:
	rustup target add x86_64-pc-windows-gnu
	cargo build --release && mv mv ./target/x86_64-pc-windows-gnu/release/agent.exe .

build-msvc:
	rustup target add x86_64-pc-windows-msvc
	cargo build --release --target=x86_64-pc-windows-msvc && mv mv ./target/x86_64-pc-windows-msvc/release/agent.exe .

build-gnu:
	rustup target add x86_64-pc-windows-gnu
	cargo build --release --target=x86_64-pc-windows-gnu && mv mv ./target/x86_64-pc-windows-gnu/release/agent.exe .