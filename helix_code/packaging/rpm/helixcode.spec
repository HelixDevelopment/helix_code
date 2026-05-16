Name: helixcode
Version: 3.0.0
Release: 1%{?dist}
Summary: Distributed AI Development Platform
License: Proprietary
URL: https://helixcode.dev
Source0: helixcode-%{version}.tar.gz
BuildRequires: golang >= 1.24
Requires: ca-certificates

%description
HelixCode is an enterprise-grade distributed AI development platform
that enables intelligent task division, work preservation, and
multi-provider LLM integration through a unified CLI, REST API,
and Terminal UI.

%prep
%setup -q

%build
make build

%install
mkdir -p %{buildroot}%{_bindir}
mkdir -p %{buildroot}%{_sysconfdir}/helixcode
mkdir -p %{buildroot}%{_unitdir}
mkdir -p %{buildroot}%{_sharedstatedir}/helixcode
install -m 755 bin/helixcode %{buildroot}%{_bindir}/helixcode
install -m 644 config/config.yaml %{buildroot}%{_sysconfdir}/helixcode/config.yaml.example
install -m 644 packaging/rpm/helixcode.service %{buildroot}%{_unitdir}/helixcode.service

%pre
getent group helixcode >/dev/null || groupadd -r helixcode
getent passwd helixcode >/dev/null || useradd -r -g helixcode -d /var/lib/helixcode -s /sbin/nologin helixcode
exit 0

%post
systemctl daemon-reload
systemctl enable helixcode.service || true
systemctl start helixcode.service || true

%preun
systemctl stop helixcode.service || true
systemctl disable helixcode.service || true

%files
%{_bindir}/helixcode
%{_sysconfdir}/helixcode/config.yaml.example
%{_unitdir}/helixcode.service
%dir %{_sharedstatedir}/helixcode
%doc README.md

%changelog
* Thu May 08 2026 Helix Development <dev@helix.code> - 3.0.0
- Initial RPM release
