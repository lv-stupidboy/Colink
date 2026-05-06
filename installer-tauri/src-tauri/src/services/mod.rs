pub mod registry;
pub mod disk_space;
pub mod file_ops;
pub mod shortcut;
pub mod service_manager;
pub mod dependency;
pub mod config;
pub mod uninstall;
pub mod installer;
pub mod bundle;  // Mac App Bundle operations
pub mod plist;   // Mac plist (alternative to registry)

pub use registry::*;
pub use disk_space::*;
pub use file_ops::*;
pub use shortcut::*;
pub use service_manager::*;
pub use dependency::*;
pub use config::*;
pub use uninstall::*;
pub use installer::*;
// bundle and plist exports are platform-specific, used via module path