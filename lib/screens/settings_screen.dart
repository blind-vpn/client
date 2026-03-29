import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../theme/app_theme.dart';
import '../models/vpn_state.dart';
import '../services/vpn_provider.dart';

class SettingsScreen extends StatelessWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Settings'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios_new_rounded, size: 20),
          onPressed: () => Navigator.pop(context),
        ),
      ),
      body: Consumer<VpnProvider>(
        builder: (context, vpn, _) {
          return ListView(
            padding: const EdgeInsets.all(16),
            children: [
              // Account section
              const _SectionHeader(title: 'ACCOUNT'),
              _SettingsCard(
                children: [
                  _SettingsRow(
                    label: 'Account Number',
                    value: vpn.storage.accountID ?? 'None',
                    trailing: IconButton(
                      icon: const Icon(Icons.copy_rounded, size: 18, color: AppColors.textMuted),
                      onPressed: () {
                        if (vpn.storage.accountID != null) {
                          Clipboard.setData(ClipboardData(text: vpn.storage.accountID!));
                          ScaffoldMessenger.of(context).showSnackBar(
                            const SnackBar(
                              content: Text('Account number copied'),
                              duration: Duration(seconds: 1),
                              backgroundColor: AppColors.bgElevated,
                            ),
                          );
                        }
                      },
                    ),
                  ),
                ],
              ),

              const SizedBox(height: 24),

              // Connection section
              const _SectionHeader(title: 'CONNECTION'),
              _SettingsCard(
                children: [
                  _SettingsToggle(
                    label: 'Auto-connect',
                    subtitle: 'Connect when app opens',
                    value: false,
                    onChanged: (v) {
                      // TODO: Implement auto-connect preference
                    },
                  ),
                  const Divider(color: AppColors.border, height: 1),
                  _SettingsToggle(
                    label: 'Kill Switch',
                    subtitle: 'Block traffic if VPN drops',
                    value: false,
                    onChanged: (v) {
                      // TODO: Implement kill switch preference
                    },
                  ),
                ],
              ),

              const SizedBox(height: 24),

              // API section
              const _SectionHeader(title: 'ADVANCED'),
              _SettingsCard(
                children: [
                  _SettingsRow(
                    label: 'API Server',
                    value: vpn.storage.apiURL,
                  ),
                ],
              ),

              const SizedBox(height: 32),

              // Logout
              SizedBox(
                height: 48,
                child: OutlinedButton(
                  onPressed: () async {
                    if (vpn.status != ConnectionStatus.disconnected) {
                      await vpn.disconnect();
                    }
                    await vpn.storage.clear();
                    if (context.mounted) {
                      Navigator.of(context).popUntil((route) => route.isFirst);
                    }
                  },
                  style: OutlinedButton.styleFrom(
                    side: const BorderSide(color: AppColors.error, width: 1),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                  ),
                  child: const Text('Log Out', style: TextStyle(color: AppColors.error, fontSize: 15)),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _SectionHeader extends StatelessWidget {
  final String title;
  const _SectionHeader({required this.title});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(left: 4, bottom: 8),
      child: Text(
        title,
        style: const TextStyle(
          fontSize: 12,
          fontWeight: FontWeight.w600,
          color: AppColors.textMuted,
          letterSpacing: 1.2,
        ),
      ),
    );
  }
}

class _SettingsCard extends StatelessWidget {
  final List<Widget> children;
  const _SettingsCard({required this.children});

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: AppColors.border),
      ),
      child: Column(children: children),
    );
  }
}

class _SettingsRow extends StatelessWidget {
  final String label;
  final String value;
  final Widget? trailing;
  const _SettingsRow({required this.label, required this.value, this.trailing});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label, style: const TextStyle(fontSize: 15, color: AppColors.textPrimary)),
                const SizedBox(height: 2),
                Text(
                  value,
                  style: const TextStyle(fontSize: 13, color: AppColors.textMuted, fontFamily: 'monospace'),
                ),
              ],
            ),
          ),
          if (trailing != null) trailing!,
        ],
      ),
    );
  }
}

class _SettingsToggle extends StatelessWidget {
  final String label;
  final String subtitle;
  final bool value;
  final ValueChanged<bool> onChanged;

  const _SettingsToggle({
    required this.label,
    required this.subtitle,
    required this.value,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label, style: const TextStyle(fontSize: 15, color: AppColors.textPrimary)),
                Text(subtitle, style: const TextStyle(fontSize: 13, color: AppColors.textMuted)),
              ],
            ),
          ),
          Switch(
            value: value,
            onChanged: onChanged,
            activeColor: AppColors.accent,
            inactiveTrackColor: AppColors.bgElevated,
          ),
        ],
      ),
    );
  }
}
