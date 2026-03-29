import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../theme/app_theme.dart';
import '../models/vpn_state.dart';
import '../services/vpn_provider.dart';

class ServerListScreen extends StatelessWidget {
  const ServerListScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Servers'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios_new_rounded, size: 20),
          onPressed: () => Navigator.pop(context),
        ),
      ),
      body: Consumer<VpnProvider>(
        builder: (context, vpn, _) {
          if (vpn.servers.isEmpty) {
            return const Center(
              child: CircularProgressIndicator(color: AppColors.accent),
            );
          }

          // Group by region
          final grouped = <String, List<ServerInfo>>{};
          for (final s in vpn.servers) {
            grouped.putIfAbsent(s.regionCode, () => []).add(s);
          }

          return RefreshIndicator(
            onRefresh: vpn.refreshServers,
            color: AppColors.accent,
            child: ListView(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              children: grouped.entries.expand((entry) {
                final regionName = entry.value.first.regionCity;
                return [
                  Padding(
                    padding: const EdgeInsets.only(left: 4, top: 16, bottom: 8),
                    child: Text(
                      regionName.toUpperCase(),
                      style: const TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: AppColors.textMuted,
                        letterSpacing: 1.2,
                      ),
                    ),
                  ),
                  ...entry.value.map((server) => _ServerTile(
                        server: server,
                        isSelected: vpn.selectedServer?.id == server.id,
                        onTap: () {
                          vpn.selectServer(server);
                          Navigator.pop(context, server);
                        },
                      )),
                ];
              }).toList(),
            ),
          );
        },
      ),
    );
  }
}

class _ServerTile extends StatelessWidget {
  final ServerInfo server;
  final bool isSelected;
  final VoidCallback onTap;

  const _ServerTile({required this.server, required this.isSelected, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        margin: const EdgeInsets.only(bottom: 4),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        decoration: BoxDecoration(
          color: isSelected ? AppColors.accent.withOpacity(0.1) : AppColors.bgCard,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: isSelected ? AppColors.accent.withOpacity(0.4) : AppColors.border,
          ),
        ),
        child: Row(
          children: [
            Text(server.flag, style: const TextStyle(fontSize: 22)),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    server.hostname,
                    style: const TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w500,
                      color: AppColors.textPrimary,
                    ),
                  ),
                  Text(
                    '${server.regionCity}, ${server.regionCountry}',
                    style: const TextStyle(fontSize: 13, color: AppColors.textMuted),
                  ),
                ],
              ),
            ),
            // Load indicator
            _LoadBadge(load: server.load),
            if (isSelected) ...[
              const SizedBox(width: 8),
              const Icon(Icons.check_circle, color: AppColors.accent, size: 20),
            ],
          ],
        ),
      ),
    );
  }
}

class _LoadBadge extends StatelessWidget {
  final double load;
  const _LoadBadge({required this.load});

  @override
  Widget build(BuildContext context) {
    Color color;
    if (load < 0.5) {
      color = AppColors.connected;
    } else if (load < 0.8) {
      color = const Color(0xFFF59E0B);
    } else {
      color = AppColors.error;
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: color.withOpacity(0.1),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(
        '${(load * 100).toInt()}%',
        style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: color),
      ),
    );
  }
}
