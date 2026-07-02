//go:build windows && cgo

/*
 * cgo_core_epoch_windows.cpp — extra OpenVPN3 core TU.
 *
 * OpenVPN3 3.11.6 moved the epoch data-channel implementation
 * (openvpn::DataChannelEpoch, EpochDataChannelCryptoContext) out of the
 * ovpncli.cpp single-TU into its own source file
 * openvpn/crypto/data_epoch.cpp. ovpncli.cpp does NOT include it, so without
 * this TU the link fails with undefined references to
 * DataChannelEpoch::DataChannelEpoch / check_send_iterate / calculate_iv /
 * lookup_decrypt_key / replace_update_recv_key. Compile it as its own object.
 *
 * Visibility/platform defines mirror cgo_bridge.cpp so the openvpn:: types
 * have a consistent ABI across TUs.
 */

#ifndef OPENVPN_CORE_API_VISIBILITY_HIDDEN
#define OPENVPN_CORE_API_VISIBILITY_HIDDEN
#endif

#include <openvpn/common/platform.hpp>
#include <openvpn/crypto/data_epoch.cpp>
