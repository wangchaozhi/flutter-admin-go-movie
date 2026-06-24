import 'dart:convert';

import 'package:http/http.dart' as http;

import 'session.dart';

/// Thrown when the server reply cannot be turned into a JSON object (empty body
/// or non-JSON). Its [toString] is the user-facing message, so it surfaces a
/// clear reason instead of a cryptic `FormatException` from `jsonDecode`.
class ApiException implements Exception {
  final String message;
  const ApiException(this.message);

  @override
  String toString() => message;
}

class ApiClient {
  // Default targets the backend on the same machine. Android emulator
  // (10.0.2.2), a physical device (LAN IP) or web/desktop pointing elsewhere
  // should override this with --dart-define=API_BASE_URL=...
  static const String baseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://localhost:8080',
  );

  Future<Map<String, dynamic>> post(
    String path,
    Map<String, dynamic> body,
  ) async {
    final uri = Uri.parse('$baseUrl$path');
    final response = await http.post(
      uri,
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode(body),
    );
    return _decode(response, uri);
  }

  Future<Map<String, dynamic>> get(String path) async {
    final uri = Uri.parse('$baseUrl$path');
    final response = await http.get(uri);
    return _decode(response, uri);
  }

  Future<Map<String, dynamic>> getAuth(String path) async {
    final token = await Session.token();
    final headers = <String, String>{};
    if (token != null) headers['Authorization'] = 'Bearer $token';
    final uri = Uri.parse('$baseUrl$path');
    final response = await http.get(uri, headers: headers);
    return _decode(response, uri);
  }

  Future<Map<String, dynamic>> postAuth(
    String path,
    Map<String, dynamic> body,
  ) async {
    final token = await Session.token();
    final headers = <String, String>{'Content-Type': 'application/json'};
    if (token != null) headers['Authorization'] = 'Bearer $token';
    final uri = Uri.parse('$baseUrl$path');
    final response = await http.post(
      uri,
      headers: headers,
      body: jsonEncode(body),
    );
    return _decode(response, uri);
  }

  Future<Map<String, dynamic>> putAuth(
    String path,
    Map<String, dynamic> body,
  ) async {
    final token = await Session.token();
    final headers = <String, String>{'Content-Type': 'application/json'};
    if (token != null) headers['Authorization'] = 'Bearer $token';
    final uri = Uri.parse('$baseUrl$path');
    final response = await http.put(
      uri,
      headers: headers,
      body: jsonEncode(body),
    );
    return _decode(response, uri);
  }

  Future<Map<String, dynamic>> deleteAuth(String path) async {
    final token = await Session.token();
    final headers = <String, String>{};
    if (token != null) headers['Authorization'] = 'Bearer $token';
    final uri = Uri.parse('$baseUrl$path');
    final response = await http.delete(uri, headers: headers);
    return _decode(response, uri);
  }

  /// Turns a response body into a JSON map, or throws an [ApiException] with a
  /// clear message. Guards against the common case of an empty / non-JSON body
  /// (e.g. wrong base URL, a proxy error page, or a gateway timeout), which
  /// would otherwise surface as `FormatException: Unexpected end of input`.
  Map<String, dynamic> _decode(http.Response response, Uri uri) {
    final body = response.body.trim();
    if (body.isEmpty) {
      throw ApiException(
        '服务器无响应 (HTTP ${response.statusCode})，请检查服务器地址：$uri',
      );
    }
    final dynamic decoded;
    try {
      decoded = jsonDecode(body);
    } on FormatException {
      throw ApiException(
        '服务器返回了无效数据 (HTTP ${response.statusCode})，请检查服务器地址：$uri',
      );
    }
    if (decoded is Map<String, dynamic>) {
      return decoded;
    }
    throw ApiException(
      '服务器返回了非预期数据 (HTTP ${response.statusCode})：$uri',
    );
  }
}
