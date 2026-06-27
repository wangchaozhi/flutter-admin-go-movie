import 'package:flutter/material.dart';

import '../../../core/l10n/app_strings.dart';
import '../../../core/l10n/locale_controller.dart';
import '../../../models/video.dart';
import '../../auth/change_password_sheet.dart';
import '../../notifications/notifications_page.dart';
import '../../search/search_page.dart';
import '../models/home_models.dart';

class HomeTopBar extends StatelessWidget {
  const HomeTopBar({
    super.key,
    required this.username,
    required this.onOpenVip,
    required this.onLogout,
  });

  final String username;
  final VoidCallback onOpenVip;
  final VoidCallback onLogout;

  void _openProfile(BuildContext context) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF171B24),
      showDragHandle: true,
      builder: (_) => _ProfileSheet(username: username),
    );
  }

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 10, 6),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Go Movie',
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    color: Colors.white,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  username.isNotEmpty
                      ? s.t('home.greeting', {'name': username})
                      : s.t('home.greetingGuest'),
                  style: const TextStyle(
                    color: Color(0xFF9CA3AF),
                    fontSize: 13,
                  ),
                ),
              ],
            ),
          ),
          IconButton(
            tooltip: s.t('home.search'),
            icon: const Icon(Icons.search, color: Colors.white),
            onPressed: () => Navigator.of(
              context,
            ).push(MaterialPageRoute<void>(builder: (_) => const SearchPage())),
          ),
          const NotificationBell(),
          PopupMenuButton<_UserMenuAction>(
            tooltip: s.t('home.mine'),
            color: const Color(0xFF171B24),
            position: PopupMenuPosition.under,
            offset: const Offset(0, 8),
            onSelected: (action) {
              switch (action) {
                case _UserMenuAction.profile:
                  _openProfile(context);
                case _UserMenuAction.vip:
                  onOpenVip();
                case _UserMenuAction.logout:
                  onLogout();
              }
            },
            itemBuilder: (context) => [
              PopupMenuItem(
                value: _UserMenuAction.profile,
                child: _UserMenuItem(
                  icon: Icons.badge_outlined,
                  label: s.t('home.profile'),
                ),
              ),
              PopupMenuItem(
                value: _UserMenuAction.vip,
                child: _UserMenuItem(
                  icon: Icons.workspace_premium_rounded,
                  label: s.t('home.vip'),
                  vip: true,
                ),
              ),
              const PopupMenuDivider(),
              PopupMenuItem(
                value: _UserMenuAction.logout,
                child: _UserMenuItem(
                  icon: Icons.logout_rounded,
                  label: s.t('home.logout'),
                  danger: true,
                ),
              ),
            ],
            child: _AvatarButton(username: username),
          ),
        ],
      ),
    );
  }
}

enum _UserMenuAction { profile, vip, logout }

class _AvatarButton extends StatelessWidget {
  const _AvatarButton({required this.username});

  final String username;

  @override
  Widget build(BuildContext context) {
    final letter = username.trim().isEmpty
        ? '我'
        : username.trim().characters.first.toUpperCase();

    return Container(
      width: 44,
      height: 44,
      margin: const EdgeInsets.only(left: 8),
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        gradient: const LinearGradient(
          colors: [Color(0xFFF7C948), Color(0xFF25D0AB)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        border: Border.all(color: const Color(0x66FFFFFF), width: 1.5),
        boxShadow: const [
          BoxShadow(
            color: Color(0x3325D0AB),
            blurRadius: 16,
            offset: Offset(0, 8),
          ),
        ],
      ),
      alignment: Alignment.center,
      child: Text(
        letter,
        style: const TextStyle(
          color: Color(0xFF101318),
          fontSize: 18,
          fontWeight: FontWeight.w900,
        ),
      ),
    );
  }
}

class _UserMenuItem extends StatelessWidget {
  const _UserMenuItem({
    required this.icon,
    required this.label,
    this.vip = false,
    this.danger = false,
  });

  final IconData icon;
  final String label;
  final bool vip;
  final bool danger;

  @override
  Widget build(BuildContext context) {
    final color = danger ? const Color(0xFFF87171) : Colors.white;

    return Row(
      children: [
        if (vip) const _VipBadgeIcon() else Icon(icon, color: color, size: 21),
        const SizedBox(width: 12),
        Text(
          label,
          style: TextStyle(color: color, fontWeight: FontWeight.w800),
        ),
      ],
    );
  }
}

class _VipBadgeIcon extends StatelessWidget {
  const _VipBadgeIcon();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 28,
      height: 28,
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          colors: [Color(0xFFFFE082), Color(0xFFF7C948), Color(0xFFEAB308)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(8),
        boxShadow: const [
          BoxShadow(
            color: Color(0x44F7C948),
            blurRadius: 10,
            offset: Offset(0, 4),
          ),
        ],
      ),
      child: const Icon(
        Icons.workspace_premium_rounded,
        color: Color(0xFF101318),
        size: 19,
      ),
    );
  }
}

class _ProfileSheet extends StatelessWidget {
  const _ProfileSheet({required this.username});

  final String username;

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    final displayName = username.isEmpty ? s.t('profile.mobileUser') : username;

    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _AvatarButton(username: username),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        displayName,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 20,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                      const SizedBox(height: 3),
                      Text(
                        s.t('profile.memberAccount'),
                        style: const TextStyle(color: Color(0xFF9CA3AF)),
                      ),
                    ],
                  ),
                ),
                const _VipBadgeIcon(),
              ],
            ),
            const SizedBox(height: 20),
            _ProfileInfoRow(
              icon: Icons.person_outline_rounded,
              label: s.t('profile.username'),
              value: displayName,
            ),
            _ProfileInfoRow(
              icon: Icons.shield_outlined,
              label: s.t('profile.accountStatus'),
              value: s.t('common.normal'),
            ),
          ],
        ),
      ),
    );
  }
}

class _ProfileInfoRow extends StatelessWidget {
  const _ProfileInfoRow({
    required this.icon,
    required this.label,
    required this.value,
  });

  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
      decoration: BoxDecoration(
        color: const Color(0xFF101318),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Row(
        children: [
          Icon(icon, color: const Color(0xFF25D0AB), size: 20),
          const SizedBox(width: 10),
          Text(
            label,
            style: const TextStyle(
              color: Color(0xFF9CA3AF),
              fontWeight: FontWeight.w700,
            ),
          ),
          const Spacer(),
          const SizedBox(width: 12),
          Flexible(
            child: Text(
              value,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              textAlign: TextAlign.right,
              style: const TextStyle(
                color: Colors.white,
                fontWeight: FontWeight.w800,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class ProfileCenter extends StatelessWidget {
  const ProfileCenter({
    super.key,
    required this.username,
    required this.profile,
    required this.orders,
    required this.loadingProfile,
    required this.loadingOrders,
    required this.error,
    required this.videos,
    required this.historyCount,
    required this.favoriteCount,
    required this.onOpenVip,
    required this.onOpenHistory,
    required this.onOpenFavorites,
    required this.onOpenOrders,
    required this.onOpenSettings,
    required this.onRefresh,
    required this.onLogout,
  });

  final String username;
  final MobileProfile? profile;
  final List<OrderSummary> orders;
  final bool loadingProfile;
  final bool loadingOrders;
  final String error;
  final List<Video> videos;
  final int historyCount;
  final int favoriteCount;
  final VoidCallback onOpenVip;
  final VoidCallback onOpenHistory;
  final VoidCallback onOpenFavorites;
  final VoidCallback onOpenOrders;
  final VoidCallback onOpenSettings;
  final Future<void> Function() onRefresh;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    final displayName =
        profile?.displayName ??
        (username.isEmpty ? s.t('profile.mobileUser') : username);
    final isVip = profile?.isVip ?? false;
    final readyCount = videos.where((video) => video.isReady).length;
    final vipCount = videos
        .where((video) => video.isVip && !video.isFree)
        .length;

    return RefreshIndicator(
      color: const Color(0xFF25D0AB),
      onRefresh: onRefresh,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 112),
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  s.t('center.mine'),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    color: Colors.white,
                    fontWeight: FontWeight.w900,
                  ),
                ),
              ),
              IconButton(
                onPressed: onRefresh,
                icon: const Icon(Icons.refresh_rounded),
                color: Colors.white,
                tooltip: s.t('center.refresh'),
              ),
            ],
          ),
          const SizedBox(height: 12),
          _ProfileHeaderCard(
            username: displayName,
            loading: loadingProfile,
            isVip: isVip,
            vipUntilLabel: profile?.vipUntilLabel ?? '',
            error: error,
            onOpenVip: onOpenVip,
          ),
          const SizedBox(height: 14),
          _VipMemberCard(
            isVip: isVip,
            vipUntilLabel: profile?.vipUntilLabel ?? '',
            onTap: onOpenVip,
          ),
          const SizedBox(height: 14),
          _ShortcutGrid(
            items: [
              _ShortcutItem(
                Icons.history_rounded,
                s.t('center.history'),
                historyCount > 0
                    ? s.t('center.historyCount', {'n': '$historyCount'})
                    : s.t('center.watchable', {'n': '$readyCount'}),
                onOpenHistory,
              ),
              _ShortcutItem(
                Icons.bookmark_rounded,
                s.t('center.favorites'),
                favoriteCount > 0
                    ? s.t('center.favoriteFilms', {'n': '$favoriteCount'})
                    : s.t('center.noFavorites'),
                onOpenFavorites,
              ),
              _ShortcutItem(
                Icons.receipt_long_rounded,
                s.t('center.orders'),
                s.t('center.recentOrders', {'n': '${orders.length}'}),
                onOpenOrders,
              ),
              _ShortcutItem(
                Icons.settings_rounded,
                s.t('center.settings'),
                s.t('center.prefsCache'),
                onOpenSettings,
              ),
            ],
          ),
          const SizedBox(height: 14),
          _WatchSummaryCard(readyCount: readyCount, vipCount: vipCount),
          const SizedBox(height: 14),
          _RecentOrdersCard(orders: orders, loading: loadingOrders),
          const SizedBox(height: 14),
          _SettingsCard(onLogout: onLogout),
        ],
      ),
    );
  }
}

class _ProfileHeaderCard extends StatelessWidget {
  const _ProfileHeaderCard({
    required this.username,
    required this.loading,
    required this.isVip,
    required this.vipUntilLabel,
    required this.error,
    required this.onOpenVip,
  });

  final String username;
  final bool loading;
  final bool isVip;
  final String vipUntilLabel;
  final String error;
  final VoidCallback onOpenVip;

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Row(
        children: [
          _AvatarButton(username: username),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  username,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 20,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 4),
                if (loading)
                  Text(
                    s.t('center.loading'),
                    style: const TextStyle(color: Color(0xFF9CA3AF)),
                  )
                else if (error.isNotEmpty)
                  Text(
                    error,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(color: Color(0xFFF87171)),
                  )
                else
                  Text(
                    isVip
                        ? s.t('center.vipUntil', {'date': vipUntilLabel})
                        : s.t('center.normalUser'),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(color: Color(0xFF9CA3AF)),
                  ),
              ],
            ),
          ),
          if (isVip)
            const _VipBadgeIcon()
          else
            FilledButton(
              onPressed: onOpenVip,
              style: FilledButton.styleFrom(
                backgroundColor: const Color(0xFFF7C948),
                foregroundColor: const Color(0xFF101318),
                minimumSize: const Size(86, 40),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
              child: Text(s.t('center.open')),
            ),
        ],
      ),
    );
  }
}

class _VipMemberCard extends StatelessWidget {
  const _VipMemberCard({
    required this.isVip,
    required this.vipUntilLabel,
    required this.onTap,
  });

  final bool isVip;
  final String vipUntilLabel;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(8),
        onTap: onTap,
        child: Ink(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            gradient: const LinearGradient(
              colors: [Color(0xFF3B2F10), Color(0xFF171B24)],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: const Color(0x66F7C948)),
          ),
          child: Row(
            children: [
              const _VipBadgeIcon(),
              const SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      s.t('center.vipMember'),
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 17,
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      isVip
                          ? s.t('center.vipMemberUntil', {
                              'date': vipUntilLabel,
                            })
                          : s.t('center.vipDesc'),
                      style: const TextStyle(
                        color: Color(0xFFE5E7EB),
                        height: 1.35,
                      ),
                    ),
                  ],
                ),
              ),
              const Icon(Icons.chevron_right_rounded, color: Color(0xFFF7C948)),
            ],
          ),
        ),
      ),
    );
  }
}

class _ShortcutGrid extends StatelessWidget {
  const _ShortcutGrid({required this.items});

  final List<_ShortcutItem> items;

  @override
  Widget build(BuildContext context) {
    return GridView.count(
      crossAxisCount: 2,
      mainAxisSpacing: 10,
      crossAxisSpacing: 10,
      childAspectRatio: 1.7,
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      children: [for (final item in items) _ShortcutTile(item: item)],
    );
  }
}

class _ShortcutTile extends StatelessWidget {
  const _ShortcutTile({required this.item});

  final _ShortcutItem item;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(8),
        onTap: item.onTap,
        child: Ink(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: const Color(0xFF171B24),
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: const Color(0xFF2B3140)),
          ),
          child: Row(
            children: [
              Container(
                width: 34,
                height: 34,
                decoration: BoxDecoration(
                  color: const Color(0x1A25D0AB),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Icon(
                  item.icon,
                  color: const Color(0xFF25D0AB),
                  size: 20,
                ),
              ),
              const SizedBox(width: 9),
              Expanded(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      item.title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                    const SizedBox(height: 3),
                    Text(
                      item.subtitle,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: Color(0xFF9CA3AF),
                        fontSize: 12,
                      ),
                    ),
                  ],
                ),
              ),
              const Icon(
                Icons.chevron_right_rounded,
                color: Color(0xFF6B7280),
                size: 18,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ContentSheet extends StatelessWidget {
  const _ContentSheet({
    required this.title,
    required this.loading,
    required this.emptyText,
    required this.children,
  });

  final String title;
  final bool loading;
  final String emptyText;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 20,
                fontWeight: FontWeight.w900,
              ),
            ),
            const SizedBox(height: 12),
            if (loading)
              const Padding(
                padding: EdgeInsets.symmetric(vertical: 24),
                child: Center(
                  child: CircularProgressIndicator(color: Color(0xFF25D0AB)),
                ),
              )
            else if (children.isEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 18),
                child: Text(
                  emptyText,
                  style: const TextStyle(color: Color(0xFF9CA3AF)),
                ),
              )
            else
              Flexible(child: ListView(shrinkWrap: true, children: children)),
          ],
        ),
      ),
    );
  }
}

class HistorySheet extends StatelessWidget {
  const HistorySheet({
    super.key,
    required this.items,
    required this.loading,
    required this.onOpenVideo,
  });

  final List<HistoryEntry> items;
  final bool loading;
  final ValueChanged<Video> onOpenVideo;

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    return _ContentSheet(
      title: s.t('center.history'),
      loading: loading,
      emptyText: s.t('center.noHistory'),
      children: [
        for (final item in items)
          _VideoActionRow(
            video: item.video,
            subtitle: s.t('center.watched', {'progress': '${item.progress}'}),
            trailing: item.video.durationLabel,
            onTap: () {
              Navigator.pop(context);
              onOpenVideo(item.video);
            },
          ),
      ],
    );
  }
}

class FavoritesSheet extends StatelessWidget {
  const FavoritesSheet({
    super.key,
    required this.items,
    required this.loading,
    required this.onOpenVideo,
    required this.onRemove,
  });

  final List<FavoriteEntry> items;
  final bool loading;
  final ValueChanged<Video> onOpenVideo;
  final ValueChanged<Video> onRemove;

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    return _ContentSheet(
      title: s.t('center.favorites'),
      loading: loading,
      emptyText: s.t('center.noFavoriteFilms'),
      children: [
        for (final item in items)
          _VideoActionRow(
            video: item.video,
            subtitle: item.video.categoryName.isEmpty
                ? s.t('center.favorited')
                : item.video.categoryName,
            trailing: s.t('center.remove'),
            onTap: () {
              Navigator.pop(context);
              onOpenVideo(item.video);
            },
            onTrailingTap: () => onRemove(item.video),
          ),
      ],
    );
  }
}

class OrdersSheet extends StatelessWidget {
  const OrdersSheet({super.key, required this.orders, required this.loading});

  final List<OrderSummary> orders;
  final bool loading;

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    return _ContentSheet(
      title: s.t('center.orders'),
      loading: loading,
      emptyText: s.t('center.noOrders'),
      children: [for (final order in orders) _OrderRow(order: order)],
    );
  }
}

class SettingsSheet extends StatefulWidget {
  const SettingsSheet({super.key, required this.setting, required this.onSave});

  final MobileSetting setting;
  final Future<void> Function(MobileSetting setting) onSave;

  @override
  State<SettingsSheet> createState() => _SettingsSheetState();
}

class _SettingsSheetState extends State<SettingsSheet> {
  late MobileSetting _setting = widget.setting;
  bool _saving = false;

  Future<void> _save(MobileSetting setting) async {
    setState(() {
      _setting = setting;
      _saving = true;
    });
    await widget.onSave(setting);
    if (mounted) setState(() => _saving = false);
  }

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    final currentCode = AppLocale.codeOf(Localizations.localeOf(context));
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              s.t('settings.title'),
              style: const TextStyle(
                color: Colors.white,
                fontSize: 20,
                fontWeight: FontWeight.w900,
              ),
            ),
            const SizedBox(height: 12),
            SwitchListTile.adaptive(
              value: _setting.autoPlay,
              activeThumbColor: const Color(0xFF25D0AB),
              contentPadding: EdgeInsets.zero,
              title: Text(
                s.t('settings.autoPlay'),
                style: const TextStyle(color: Colors.white),
              ),
              subtitle: Text(
                s.t('settings.autoPlaySub'),
                style: const TextStyle(color: Color(0xFF9CA3AF)),
              ),
              onChanged: _saving
                  ? null
                  : (value) => _save(_setting.copyWith(autoPlay: value)),
            ),
            SwitchListTile.adaptive(
              value: _setting.wifiOnly,
              activeThumbColor: const Color(0xFF25D0AB),
              contentPadding: EdgeInsets.zero,
              title: Text(
                s.t('settings.wifiOnly'),
                style: const TextStyle(color: Colors.white),
              ),
              subtitle: Text(
                s.t('settings.wifiOnlySub'),
                style: const TextStyle(color: Color(0xFF9CA3AF)),
              ),
              onChanged: _saving
                  ? null
                  : (value) => _save(_setting.copyWith(wifiOnly: value)),
            ),
            const SizedBox(height: 8),
            DropdownButtonFormField<String>(
              initialValue: _setting.preferredQuality,
              dropdownColor: const Color(0xFF171B24),
              decoration: InputDecoration(
                labelText: s.t('settings.quality'),
                labelStyle: const TextStyle(color: Color(0xFF9CA3AF)),
                enabledBorder: const OutlineInputBorder(
                  borderSide: BorderSide(color: Color(0xFF2B3140)),
                ),
                focusedBorder: const OutlineInputBorder(
                  borderSide: BorderSide(color: Color(0xFF25D0AB)),
                ),
              ),
              style: const TextStyle(color: Colors.white),
              items: [
                DropdownMenuItem(
                  value: 'auto',
                  child: Text(s.t('settings.qualityAuto')),
                ),
                const DropdownMenuItem(value: '720p', child: Text('720p')),
                const DropdownMenuItem(value: '1080p', child: Text('1080p')),
              ],
              onChanged: _saving || _setting.preferredQuality.isEmpty
                  ? null
                  : (value) => _save(
                      _setting.copyWith(preferredQuality: value ?? 'auto'),
                    ),
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<String>(
              initialValue: currentCode,
              dropdownColor: const Color(0xFF171B24),
              decoration: InputDecoration(
                labelText: s.t('settings.language'),
                labelStyle: const TextStyle(color: Color(0xFF9CA3AF)),
                prefixIcon: const Icon(
                  Icons.language_rounded,
                  color: Color(0xFF25D0AB),
                ),
                enabledBorder: const OutlineInputBorder(
                  borderSide: BorderSide(color: Color(0xFF2B3140)),
                ),
                focusedBorder: const OutlineInputBorder(
                  borderSide: BorderSide(color: Color(0xFF25D0AB)),
                ),
              ),
              style: const TextStyle(color: Colors.white),
              items: [
                for (final item in AppLocale.values)
                  DropdownMenuItem(value: item.code, child: Text(item.label)),
              ],
              onChanged: (code) {
                if (code != null) {
                  localeController.setLocale(AppLocale.byCode(code).locale);
                }
              },
            ),
            const SizedBox(height: 8),
            ListTile(
              contentPadding: EdgeInsets.zero,
              leading: const Icon(
                Icons.lock_reset_rounded,
                color: Color(0xFF25D0AB),
              ),
              title: Text(
                s.t('settings.changePassword'),
                style: const TextStyle(color: Colors.white),
              ),
              subtitle: Text(
                s.t('settings.changePasswordSub'),
                style: const TextStyle(color: Color(0xFF9CA3AF)),
              ),
              trailing: const Icon(
                Icons.chevron_right_rounded,
                color: Color(0xFF9CA3AF),
              ),
              onTap: () => showChangePasswordSheet(context),
            ),
          ],
        ),
      ),
    );
  }
}

class _ShortcutItem {
  const _ShortcutItem(this.icon, this.title, this.subtitle, this.onTap);

  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;
}

class _VideoActionRow extends StatelessWidget {
  const _VideoActionRow({
    required this.video,
    required this.subtitle,
    required this.trailing,
    required this.onTap,
    this.onTrailingTap,
  });

  final Video video;
  final String subtitle;
  final String trailing;
  final VoidCallback onTap;
  final VoidCallback? onTrailingTap;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      contentPadding: EdgeInsets.zero,
      onTap: onTap,
      leading: ClipRRect(
        borderRadius: BorderRadius.circular(8),
        child: SizedBox(
          width: 58,
          height: 42,
          child: video.fullCoverUrl.isEmpty
              ? const ColoredBox(color: Color(0xFF202532))
              : Image.network(
                  video.fullCoverUrl,
                  fit: BoxFit.cover,
                  errorBuilder: (_, _, _) =>
                      const ColoredBox(color: Color(0xFF202532)),
                ),
        ),
      ),
      title: Text(
        video.title,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: const TextStyle(
          color: Colors.white,
          fontWeight: FontWeight.w800,
        ),
      ),
      subtitle: Text(
        subtitle,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: const TextStyle(color: Color(0xFF9CA3AF)),
      ),
      trailing: TextButton(
        onPressed: onTrailingTap,
        child: Text(
          trailing,
          style: const TextStyle(
            color: Color(0xFFF7C948),
            fontWeight: FontWeight.w800,
          ),
        ),
      ),
    );
  }
}

class _WatchSummaryCard extends StatelessWidget {
  const _WatchSummaryCard({required this.readyCount, required this.vipCount});

  final int readyCount;
  final int vipCount;

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    return _SectionCard(
      title: s.t('center.history'),
      trailing: s.t('center.continueWatch'),
      child: Row(
        children: [
          Expanded(
            child: _MetricBox(
              label: s.t('center.watchableShort'),
              value: '$readyCount',
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: _MetricBox(
              label: s.t('center.vipExclusive'),
              value: '$vipCount',
            ),
          ),
        ],
      ),
    );
  }
}

class _RecentOrdersCard extends StatelessWidget {
  const _RecentOrdersCard({required this.orders, required this.loading});

  final List<OrderSummary> orders;
  final bool loading;

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    return _SectionCard(
      title: s.t('center.orders'),
      trailing: loading
          ? s.t('center.loadingShort')
          : s.t('center.recentOrder'),
      child: loading
          ? const Padding(
              padding: EdgeInsets.symmetric(vertical: 16),
              child: Center(
                child: CircularProgressIndicator(color: Color(0xFF25D0AB)),
              ),
            )
          : orders.isEmpty
          ? Padding(
              padding: const EdgeInsets.symmetric(vertical: 12),
              child: Text(
                s.t('center.noOrders'),
                style: const TextStyle(color: Color(0xFF9CA3AF)),
              ),
            )
          : Column(
              children: [for (final order in orders) _OrderRow(order: order)],
            ),
    );
  }
}

class _OrderRow extends StatelessWidget {
  const _OrderRow({required this.order});

  final OrderSummary order;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Row(
        children: [
          const Icon(
            Icons.receipt_long_rounded,
            color: Color(0xFFF7C948),
            size: 22,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  order.productName,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  order.orderNo,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Color(0xFF9CA3AF),
                    fontSize: 12,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 10),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Text(
                order.priceLabel,
                style: const TextStyle(
                  color: Colors.white,
                  fontWeight: FontWeight.w900,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                order.statusLabel,
                style: TextStyle(
                  color: order.statusColor,
                  fontSize: 12,
                  fontWeight: FontWeight.w800,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _SettingsCard extends StatelessWidget {
  const _SettingsCard({required this.onLogout});

  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    return _SectionCard(
      title: s.t('settings.title'),
      trailing: s.t('settings.basic'),
      child: Column(
        children: [
          _SettingRow(
            icon: Icons.cleaning_services_outlined,
            label: s.t('settings.clearCache'),
            value: s.t('settings.clearCacheValue'),
          ),
          _SettingRow(
            icon: Icons.info_outline_rounded,
            label: s.t('settings.about'),
            value: s.t('center.aboutValue'),
          ),
          const SizedBox(height: 8),
          OutlinedButton.icon(
            onPressed: onLogout,
            icon: const Icon(Icons.logout_rounded),
            label: Text(s.t('settings.logout')),
            style: OutlinedButton.styleFrom(
              foregroundColor: const Color(0xFFF87171),
              side: const BorderSide(color: Color(0x66F87171)),
              minimumSize: const Size.fromHeight(44),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SettingRow extends StatelessWidget {
  const _SettingRow({
    required this.icon,
    required this.label,
    required this.value,
  });

  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        children: [
          Icon(icon, color: const Color(0xFF25D0AB), size: 20),
          const SizedBox(width: 10),
          Text(
            label,
            style: const TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.w800,
            ),
          ),
          const Spacer(),
          Text(value, style: const TextStyle(color: Color(0xFF9CA3AF))),
        ],
      ),
    );
  }
}

class _SectionCard extends StatelessWidget {
  const _SectionCard({
    required this.title,
    required this.trailing,
    required this.child,
  });

  final String title;
  final String trailing;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  title,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.w900,
                  ),
                ),
              ),
              Text(
                trailing,
                style: const TextStyle(
                  color: Color(0xFF9CA3AF),
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          child,
        ],
      ),
    );
  }
}

class _MetricBox extends StatelessWidget {
  const _MetricBox({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
      decoration: BoxDecoration(
        color: const Color(0xFF101318),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            value,
            style: const TextStyle(
              color: Color(0xFFF7C948),
              fontSize: 22,
              fontWeight: FontWeight.w900,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            label,
            style: const TextStyle(color: Color(0xFF9CA3AF), fontSize: 12),
          ),
        ],
      ),
    );
  }
}
