pub mod error;
pub mod store;
pub mod commands;
pub mod services;

use tauri::Manager;
use tauri_plugin_dialog::DialogExt;
use store::{AppState, AppMode, detect_app_mode};

/// Run the Tauri application
pub fn run() {
    // Detect app mode from exe filename
    let mode = detect_app_mode();

    tauri::Builder::default()
        // Plugins
        .plugin(tauri_plugin_log::Builder::new().build())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_store::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            // Focus existing window when second instance launched
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.set_focus();
            }
        }))

        // State management
        .manage(AppState::new(mode))

        // Commands
        .invoke_handler(tauri::generate_handler![
            // Mode commands
            commands::is_launcher_mode,
            commands::get_startup_action,
            commands::get_app_path,
            commands::get_install_dir,
            commands::get_resource_path,
            commands::get_version,

            // Installation commands
            commands::check_installed,
            commands::check_old_isdp,
            commands::uninstall_old_isdp,
            commands::select_directory,
            commands::get_disk_space,
            commands::generate_config_preview,
            commands::read_existing_config,
            commands::start_installation,
            commands::create_shortcut,

            // Uninstall commands
            commands::confirm_uninstall,
            commands::run_uninstall,
            commands::clean_registry,
            commands::remove_shortcuts,
            commands::uninstall,

            // Service commands
            commands::start_service,
            commands::stop_service,
            commands::get_service_status,
            commands::get_running_agents,

            // Dependency commands
            commands::check_dependency,
            commands::install_dependency,
            commands::check_all_dependencies,

            // Config commands
            commands::read_config_file,
            commands::save_config,
            commands::get_existing_config,

            // Launcher commands
            commands::open_logs,
            commands::open_data_dir,
            commands::open_config,
            commands::open_console,

            // Invite commands
            commands::verify_invite_code,
            commands::save_invite_code,
            commands::load_invite_code,
            commands::get_system_username,

            // Window commands
            commands::window_minimize,
            commands::window_maximize,
            commands::window_close,
        ])

        .setup(|app| {
            let state = app.state::<AppState>();
            let mode = state.get_mode();

            // Load installer config - try multiple paths
            let exe_path = std::env::current_exe().unwrap_or_default();
            let exe_dir = exe_path.parent().unwrap_or(std::path::Path::new("."));

            let config_paths = vec![
                // 1. Standard resource_dir (packaged app)
                app.path().resource_dir().unwrap_or_default().join("installer-config.json"),
                // 2. exe_dir/resources (dev/test mode)
                exe_dir.join("resources").join("installer-config.json"),
                // 3. exe_dir/../resources (dev mode from target/release)
                exe_dir.join("..").join("resources").join("installer-config.json"),
            ];

            let mut config_loaded = false;
            for config_path in config_paths {
                log::info!("Trying config path: {:?}", config_path);
                if config_path.exists() {
                    if let Err(e) = state.load_installer_config(&config_path) {
                        log::warn!("Failed to load installer config from {:?}: {}", config_path, e);
                    } else {
                        log::info!("Installer config loaded successfully from {:?}", config_path);
                        config_loaded = true;
                        break;
                    }
                }
            }

            if !config_loaded {
                log::warn!("Installer config file not found in any location");
            }

            // Setup based on mode
            match mode {
                AppMode::Launcher => {
                    log::info!("Running in Launcher mode");
                    // Check if installed
                    if let Ok(installed) = services::registry::get_installed_version() {
                        log::info!("Registry check result: installed={}, install_dir={:?}",
                            installed.installed, installed.install_dir);
                        if !installed.installed {
                            // Show error and exit
                            app.dialog()
                                .message("Colink 未安装，请先运行安装程序")
                                .title("错误")
                                .kind(tauri_plugin_dialog::MessageDialogKind::Error);
                            std::process::exit(1);
                        }
                        // Set install dir from registry
                        if let Some(dir) = installed.install_dir {
                            log::info!("Setting install_dir to: {}", dir);
                            state.set_install_dir(dir);
                        } else {
                            log::warn!("No install_dir found in registry");
                        }
                    } else {
                        log::warn!("Failed to read registry for installed version");
                    }
                }
                AppMode::Setup => {
                    // Setup mode - check existing installation for upgrade/uninstall options
                    log::info!("Running in Setup mode");
                }
            }

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}