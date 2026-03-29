import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../theme/app_theme.dart';
import '../models/vpn_state.dart';
import '../services/vpn_provider.dart';
import 'server_list_screen.dart';
import 'settings_screen.dart';

class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 24),
          child: Consumer<VpnProvider>(
            builder: (context, vpn, _) {
              return Column(
                children: [
                  const SizedBox(height: 16),
                  _buildTopBar(context, vpn),
                  const Spacer(flex: 2),
                  _buildConnectButton(context, vpn),
                  const SizedBox(height: 24),
                  _buildStatusText(context, vpn),
                  const Spacer(flex: 1),
                  _buildServerSelector(context, vpn),
                  const SizedBox(height: 16),
                  if (vpn.error != null) _buildError(context, vpn.error!),
                  const Spacer(flex: 2),
                ],
              );
            },
          ),
        ),
      ),
    );
  }

  Widget _buildTopBar(BuildContext context, VpnProvider vpn) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text('VPN', style: Theme.of(context).textTheme.headlineMedium),
        IconButton(
          icon: const Icon(Icons.settings_outlined, color: AppColors.textSecondary),
          onPressed: () => Navigator.push(
            context,
            MaterialPageRoute(builder: (_) => const SettingsScreen()),
          ),
        ),
      ],
    );
  }

  Widget _buildConnectButton(BuildContext context, VpnProvider vpn) {
    final isConnected = vpn.status == ConnectionStatus.connected;
    final isTransitioning = vpn.status == ConnectionStatus.connecting ||
        vpn.status == ConnectionStatus.disconnecting;

    final color = isConnected ? AppColors.connected : AppColors.accent;
    const size = 180.0;

    return GestureDetector(
      onTap: isTransitioning
          ? null
          : () {
              if (isConnected) {
                vpn.disconnect();
              } else {
                vpn.connect();
              }
            },
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 300),
        width: size,
        height: size,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: color.withOpacity(0.1),
          border: Border.all(
            color: isTransitioning ? color.withOpacity(0.3) : color.withOpacity(0.6),
            width: 3,
          ),
          boxShadow: isConnected
              ? [BoxShadow(color: color.withOpacity(0.2), blurRadius: 40, spreadRadius: 8)]
              : [],
        ),
        child: Center(
          child: isTransitioning
              ? SizedBox(
                  width: 40,
                  height: 40,
                  child: CircularProgressIndicator(strokeWidth: 3, color: color),
                )
              : Icon(
                  Icons.power_settings_new_rounded,
                  size: 64,
                  color: color,
                ),
        ),
      ),
    );
  }

  Widget _buildStatusText(BuildContext context, VpnProvider vpn) {
    String label;
    Color color;

    switch (vpn.status) {
      case ConnectionStatus.disconnected:
        label = 'Not Connected';
        color = AppColors.textMuted;
      case ConnectionStatus.connecting:
        label = 'Connecting...';
        color = AppColors.accent;
      case ConnectionStatus.connected:
        label = 'Connected';
        color = AppColors.connected;
      case ConnectionStatus.disconnecting:
        label = 'Disconnecting...';
        color = AppColors.accent;
    }

    return Column(
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              width: 8,
              height: 8,
              decoration: BoxDecoration(shape: BoxShape.circle, color: color),
            ),
            const SizedBox(width: 8),
            Text(label, style: TextStyle(fontSize: 16, fontWeight: FontWeight.w500, color: color)),
          ],
        ),
        if (vpn.status == ConnectionStatus.connected && vpn.selectedServer != null) ...[
          const SizedBox(height: 4),
          Text(
            '${vpn.selectedServer!.regionCity}, ${vpn.selectedServer!.regionCountry}',
            style: Theme.of(context).textTheme.bodyMedium,
          ),
        ],
      ],
    );
  }

  Widget _buildServerSelector(BuildContext context, VpnProvider vpn) {
    return GestureDetector(
      onTap: () async {
        final server = await Navigator.push<ServerInfo>(
          context,
          MaterialPageRoute(builder: (_) => const ServerListScreen()),
        );
        if (server != null) {
          vpn.selectServer(server);
        }
      },
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16),
          border: Border.all(color: AppColors.border),
        ),
        child: Row(
          children: [
            if (vpn.selectedServer != null) ...[
              Text(vpn.selectedServer!.flag, style: const TextStyle(fontSize: 24)),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      vpn.selectedServer!.regionCity,
                      style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600, color: AppColors.textPrimary),
                    ),
                    Text(
                      vpn.selectedServer!.hostname,
                      style: const TextStyle(fontSize: 13, color: AppColors.textMuted),
                    ),
                  ],
                ),
              ),
            ] else ...[
              const Icon(Icons.public, color: AppColors.textSecondary, size: 28),
              const SizedBox(width: 12),
              const Expanded(
                child: Text(
                  'Select a server',
                  style: TextStyle(fontSize: 16, color: AppColors.textSecondary),
                ),
              ),
            ],
            const Icon(Icons.chevron_right_rounded, color: AppColors.textMuted),
          ],
        ),
      ),
    );
  }

  Widget _buildError(BuildContext context, String error) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.error.withOpacity(0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.error.withOpacity(0.3)),
      ),
      child: Text(error, style: const TextStyle(color: AppColors.error, fontSize: 13)),
    );
  }
}
