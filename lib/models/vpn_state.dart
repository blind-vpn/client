enum ConnectionStatus { disconnected, connecting, connected, disconnecting }

class ServerInfo {
  final String id;
  final String hostname;
  final String publicIP;
  final String wgPubkey;
  final int wgPort;
  final String regionCode;
  final String regionCity;
  final String regionCountry;
  final String status;
  final double load;

  ServerInfo({
    required this.id,
    required this.hostname,
    required this.publicIP,
    required this.wgPubkey,
    required this.wgPort,
    required this.regionCode,
    required this.regionCity,
    required this.regionCountry,
    required this.status,
    required this.load,
  });

  factory ServerInfo.fromJson(Map<String, dynamic> json) {
    final region = json['region'] as Map<String, dynamic>;
    return ServerInfo(
      id: json['id'] as String,
      hostname: json['hostname'] as String,
      publicIP: json['public_ip'] as String,
      wgPubkey: json['wg_pubkey'] as String,
      wgPort: json['wg_port'] as int,
      regionCode: region['code'] as String,
      regionCity: region['city'] as String,
      regionCountry: region['country'] as String,
      status: json['status'] as String,
      load: (json['load'] as num).toDouble(),
    );
  }

  String get flag => _countryToFlag(regionCountry);

  static String _countryToFlag(String countryCode) {
    final codes = countryCode.toUpperCase().codeUnits;
    if (codes.length != 2) return '';
    return String.fromCharCode(codes[0] + 0x1F1A5) +
        String.fromCharCode(codes[1] + 0x1F1A5);
  }
}

class KeyRegistration {
  final String id;
  final String serverID;
  final String serverIP;
  final String serverPubkey;
  final int serverPort;
  final String allowedIP;

  KeyRegistration({
    required this.id,
    required this.serverID,
    required this.serverIP,
    required this.serverPubkey,
    required this.serverPort,
    required this.allowedIP,
  });

  factory KeyRegistration.fromJson(Map<String, dynamic> json) {
    return KeyRegistration(
      id: json['id'] as String,
      serverID: json['server_id'] as String,
      serverIP: json['server_ip'] as String,
      serverPubkey: json['server_pubkey'] as String,
      serverPort: json['server_port'] as int,
      allowedIP: json['allowed_ip'] as String,
    );
  }
}
