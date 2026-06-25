import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import '../../core/session.dart';
import 'login_storage.dart';
import 'widgets/login_header.dart';

class MobileLoginPage extends StatefulWidget {
  const MobileLoginPage({super.key});

  @override
  State<MobileLoginPage> createState() => _MobileLoginPageState();
}

class _MobileLoginPageState extends State<MobileLoginPage> {
  final _storage = LoginStorage();
  final _usernameController = TextEditingController(text: 'user');
  final _passwordController = TextEditingController(text: '123456');

  bool _loading = false;
  bool _remember = true;
  bool _ready = false;
  String _usernameError = '';
  String _passwordError = '';

  @override
  void initState() {
    super.initState();
    _loadSavedLogin();
  }

  @override
  void dispose() {
    _usernameController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _loadSavedLogin() async {
    final saved = await _storage.load();
    if (!mounted) return;

    _usernameController.text = saved.username;
    _passwordController.text = saved.password;
    setState(() {
      _remember = saved.remember;
      _ready = true;
    });
  }

  Future<void> _login() async {
    if (!_validate()) return;

    setState(() => _loading = true);
    try {
      final username = _usernameController.text.trim();
      final password = _passwordController.text;
      final resp = await ApiClient().post('/api/mobile/login', {
        'username': username,
        'password': password,
      });

      if (!mounted) return;
      if (resp['code'] != 0) {
        _showMessage(resp['msg']?.toString() ?? '登录失败');
        return;
      }

      final data = resp['data'] as Map<String, dynamic>?;
      final token = data?['token'] as String? ?? '';
      await Session.save(token, username);
      await _storage.save(
        username: username,
        password: password,
        remember: _remember,
      );
      if (!mounted) return;
      Navigator.pushReplacementNamed(context, '/home');
    } catch (e) {
      if (!mounted) return;
      _showMessage('登录失败: $e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  bool _validate() {
    final usernameError = _usernameController.text.trim().isEmpty
        ? '请输入账号'
        : '';
    final passwordError = _passwordController.text.isEmpty ? '请输入登录密码' : '';
    setState(() {
      _usernameError = usernameError;
      _passwordError = passwordError;
    });
    return usernameError.isEmpty && passwordError.isEmpty;
  }

  void _showMessage(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), behavior: SnackBarBehavior.floating),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (!_ready) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }

    return Scaffold(
      backgroundColor: const Color(0xFF0D0F14),
      body: Stack(
        fit: StackFit.expand,
        children: [
          const _LoginBackground(),
          SafeArea(
            child: Center(
              child: SingleChildScrollView(
                padding: const EdgeInsets.fromLTRB(20, 24, 20, 32),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 430),
                  child: _LoginCard(
                    usernameController: _usernameController,
                    passwordController: _passwordController,
                    usernameError: _usernameError,
                    passwordError: _passwordError,
                    remember: _remember,
                    loading: _loading,
                    onRememberChanged: (value) =>
                        setState(() => _remember = value),
                    onLogin: _login,
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _LoginBackground extends StatelessWidget {
  const _LoginBackground();

  @override
  Widget build(BuildContext context) {
    return const DecoratedBox(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [Color(0xFF101318), Color(0xFF171B24), Color(0xFF10251F)],
          begin: Alignment.topCenter,
          end: Alignment.bottomRight,
        ),
      ),
      child: SizedBox.expand(),
    );
  }
}

class _LoginCard extends StatelessWidget {
  const _LoginCard({
    required this.usernameController,
    required this.passwordController,
    required this.usernameError,
    required this.passwordError,
    required this.remember,
    required this.loading,
    required this.onRememberChanged,
    required this.onLogin,
  });

  final TextEditingController usernameController;
  final TextEditingController passwordController;
  final String usernameError;
  final String passwordError;
  final bool remember;
  final bool loading;
  final ValueChanged<bool> onRememberChanged;
  final VoidCallback onLogin;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: const Color(0xFF171B24),
      elevation: 18,
      shadowColor: const Color(0x99000000),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(8),
        side: const BorderSide(color: Color(0xFF2B3140)),
      ),
      child: Padding(
        padding: const EdgeInsets.all(22),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const LoginHeader(),
            const SizedBox(height: 24),
            TextField(
              controller: usernameController,
              cursorColor: const Color(0xFF25D0AB),
              style: const TextStyle(color: Colors.white),
              textInputAction: TextInputAction.next,
              decoration: _fieldDecoration(
                label: '账号',
                hint: '请输入 Go Movie 账号',
                icon: Icons.person_outline_rounded,
                error: usernameError,
              ),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: passwordController,
              cursorColor: const Color(0xFF25D0AB),
              style: const TextStyle(color: Colors.white),
              obscureText: true,
              enableSuggestions: false,
              autocorrect: false,
              textInputAction: TextInputAction.done,
              onSubmitted: (_) {
                if (!loading) onLogin();
              },
              decoration: _fieldDecoration(
                label: '密码',
                hint: '请输入登录密码',
                icon: Icons.lock_outline_rounded,
                error: passwordError,
              ),
            ),
            const SizedBox(height: 16),
            InkWell(
              borderRadius: BorderRadius.circular(8),
              onTap: loading ? null : () => onRememberChanged(!remember),
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 8),
                child: Row(
                  children: [
                    Checkbox(
                      value: remember,
                      onChanged: loading
                          ? null
                          : (value) => onRememberChanged(value ?? false),
                      activeColor: const Color(0xFF25D0AB),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(5),
                      ),
                    ),
                    const SizedBox(width: 4),
                    const Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            '记住本机登录信息',
                            style: TextStyle(
                              color: Colors.white,
                              fontWeight: FontWeight.w800,
                            ),
                          ),
                          SizedBox(height: 2),
                          Text(
                            '下次打开更快回到影片',
                            style: TextStyle(
                              color: Color(0xFF9CA3AF),
                              fontSize: 12,
                            ),
                          ),
                        ],
                      ),
                    ),
                    const Icon(
                      Icons.verified_user_outlined,
                      color: Color(0xFF25D0AB),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 18),
            FilledButton.icon(
              onPressed: loading ? null : onLogin,
              icon: loading
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Color(0xFF101318),
                      ),
                    )
                  : const Icon(Icons.arrow_forward_rounded),
              label: Text(loading ? '正在登录...' : '立即登录'),
              style: FilledButton.styleFrom(
                backgroundColor: const Color(0xFF25D0AB),
                disabledBackgroundColor: const Color(0xFF83E5D3),
                disabledForegroundColor: const Color(0xFF101318),
                foregroundColor: const Color(0xFF07110F),
                minimumSize: const Size.fromHeight(52),
                textStyle: const TextStyle(fontWeight: FontWeight.w900),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  InputDecoration _fieldDecoration({
    required String label,
    required String hint,
    required IconData icon,
    required String error,
  }) {
    return InputDecoration(
      labelText: label,
      hintText: hint,
      errorText: error.isEmpty ? null : error,
      prefixIcon: Icon(icon),
      prefixIconColor: const Color(0xFF25D0AB),
      filled: true,
      fillColor: const Color(0xFF101318),
      labelStyle: const TextStyle(color: Color(0xFF9CA3AF)),
      hintStyle: const TextStyle(color: Color(0xFF6B7280)),
      errorStyle: const TextStyle(color: Color(0xFFFCA5A5)),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: Color(0xFF2B3140)),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: Color(0xFF2B3140)),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: Color(0xFF25D0AB), width: 1.4),
      ),
      errorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: Color(0xFFF87171)),
      ),
      focusedErrorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: Color(0xFFF87171), width: 1.4),
      ),
    );
  }
}
