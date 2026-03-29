import 'dart:convert';
import 'package:http/http.dart' as http;
import '../models/vpn_state.dart';

class ApiService {
  final String baseURL;
  String? accountID;

  ApiService({required this.baseURL, this.accountID});

  Map<String, String> get _headers => {
        'Content-Type': 'application/json',
        if (accountID != null) 'Authorization': 'Account $accountID',
      };

  Future<Map<String, dynamic>> createAccount() async {
    final resp = await http.post(Uri.parse('$baseURL/v1/accounts'));
    return _decode(resp);
  }

  Future<Map<String, dynamic>> getAccount() async {
    final resp = await http.get(
      Uri.parse('$baseURL/v1/accounts/$accountID'),
      headers: _headers,
    );
    return _decode(resp);
  }

  Future<List<ServerInfo>> listServers({String? region}) async {
    var url = '$baseURL/v1/servers';
    if (region != null) url += '?region=$region';
    final resp = await http.get(Uri.parse(url), headers: _headers);
    final data = _decode(resp);
    final servers = data['servers'] as List;
    return servers.map((s) => ServerInfo.fromJson(s as Map<String, dynamic>)).toList();
  }

  Future<KeyRegistration> registerKey(String pubkey) async {
    final resp = await http.post(
      Uri.parse('$baseURL/v1/keys'),
      headers: _headers,
      body: jsonEncode({'pubkey': pubkey}),
    );
    return KeyRegistration.fromJson(_decode(resp));
  }

  Future<List<Map<String, dynamic>>> listKeys() async {
    final resp = await http.get(Uri.parse('$baseURL/v1/keys'), headers: _headers);
    final data = _decode(resp);
    return (data['keys'] as List).cast<Map<String, dynamic>>();
  }

  Future<void> revokeKey(String keyID) async {
    await http.delete(Uri.parse('$baseURL/v1/keys/$keyID'), headers: _headers);
  }

  Map<String, dynamic> _decode(http.Response resp) {
    if (resp.statusCode >= 400) {
      final body = jsonDecode(resp.body);
      throw ApiException(resp.statusCode, body['error'] ?? 'Unknown error');
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
  }
}

class ApiException implements Exception {
  final int statusCode;
  final String message;
  ApiException(this.statusCode, this.message);

  @override
  String toString() => 'API error ($statusCode): $message';
}
