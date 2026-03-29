import 'dart:io' show Platform;
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'theme/app_theme.dart';
import 'services/api_service.dart';
import 'services/vpn_service.dart';
import 'services/vpn_service_desktop.dart';
import 'services/storage_service.dart';
import 'services/vpn_provider.dart';
import 'screens/home_screen.dart';
import 'screens/login_screen.dart';

VpnTunnelService _createTunnelService() {
  if (kIsWeb) return VpnTunnelService(); // Stub for web (won't work)
  if (Platform.isLinux || Platform.isMacOS || Platform.isWindows) {
    return DesktopVpnTunnelService();
  }
  // Android/iOS use the platform channel implementation
  return VpnTunnelService();
}

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  final storage = StorageService();
  await storage.init();

  final api = ApiService(
    baseURL: storage.apiURL,
    accountID: storage.accountID,
  );

  final vpnProvider = VpnProvider(
    api: api,
    tunnel: _createTunnelService(),
    storage: storage,
  );
  await vpnProvider.init();

  runApp(
    ChangeNotifierProvider.value(
      value: vpnProvider,
      child: const VpnApp(),
    ),
  );
}

class VpnApp extends StatelessWidget {
  const VpnApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'VPN',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.dark,
      home: Consumer<VpnProvider>(
        builder: (context, vpn, _) {
          return vpn.isLoggedIn ? const HomeScreen() : const LoginScreen();
        },
      ),
    );
  }
}
