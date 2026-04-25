use thiserror::Error;

#[derive(Debug, Error)]
pub enum InstallerError {
    #[error("IO error: {context}: {source}")]
    Io {
        context: String,
        #[source]
        source: std::io::Error,
    },

    #[error("Registry error: {0}")]
    Registry(String),

    #[error("Process error: {0}")]
    Process(String),

    #[error("Config error: {0}")]
    Config(String),

    #[error("Network error: {0}")]
    Network(String),

    #[error("Installation failed: {0}")]
    InstallationFailed(String),

    #[error("Service not running")]
    ServiceNotRunning,

    #[error("Already installed at: {0}")]
    AlreadyInstalled(String),

    #[error("Not installed")]
    NotInstalled,

    #[error("Dependency not found: {0}")]
    DependencyNotFound(String),

    #[error("Invalid path: {0}")]
    InvalidPath(String),

    #[error("YAML parse error: {0}")]
    YamlParse(String),

    #[error("JSON parse error: {0}")]
    JsonParse(String),

    #[error("Permission denied: {0}")]
    PermissionDenied(String),

    #[error("Disk space insufficient: required {required}, available {available}")]
    DiskSpaceInsufficient { required: u64, available: u64 },

    #[error("Process already running: {0}")]
    ProcessAlreadyRunning(String),

    #[error("{0}")]
    Custom(String),
}

impl From<std::io::Error> for InstallerError {
    fn from(e: std::io::Error) -> Self {
        InstallerError::Io {
            context: "unknown".to_string(),
            source: e,
        }
    }
}

impl From<serde_yaml::Error> for InstallerError {
    fn from(e: serde_yaml::Error) -> Self {
        InstallerError::YamlParse(e.to_string())
    }
}

impl From<serde_json::Error> for InstallerError {
    fn from(e: serde_json::Error) -> Self {
        InstallerError::JsonParse(e.to_string())
    }
}

impl From<reqwest::Error> for InstallerError {
    fn from(e: reqwest::Error) -> Self {
        InstallerError::Network(e.to_string())
    }
}

impl From<InstallerError> for String {
    fn from(e: InstallerError) -> Self {
        e.to_string()
    }
}

pub type Result<T> = std::result::Result<T, InstallerError>;