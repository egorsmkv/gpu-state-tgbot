package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

var chatID int64

func main() {
	var (
		err              error
		token, envChatID string
	)

	token = os.Getenv("TOKEN")
	if token == "" {
		panic("TOKEN environment variable is empty")
	}

	envChatID = os.Getenv("CHAT_ID")
	if envChatID == "" {
		panic("CHAT_ID environment variable is empty")
	}
	chatID, err = strconv.ParseInt(envChatID, 10, 64)
	if err != nil {
		panic("failed to parse CHAT_ID: ")
	}

	b, err := gotgbot.NewBot(token, nil)
	if err != nil {
		panic("failed to create new bot: " + err.Error())
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occurred while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})
	updater := ext.NewUpdater(dispatcher, nil)

	dispatcher.AddHandler(handlers.NewCommand("start", start))
	dispatcher.AddHandler(handlers.NewCommand("state", state))
	dispatcher.AddHandler(handlers.NewCommand("chat_id", showChatID))

	err = updater.StartPolling(b, &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 9,
			RequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Second * 10,
			},
		},
	})
	if err != nil {
		panic("failed to start polling: " + err.Error())
	}
	log.Printf("%s has been started...\n", b.User.Username)

	updater.Idle()
}

func start(b *gotgbot.Bot, ctx *ext.Context) error {
	_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Hello, I'm @%s. I <b>send</b> information about GPU state on a server where I am connected to.", b.User.Username), &gotgbot.SendMessageOpts{
		ParseMode: "html",
	})
	if err != nil {
		return fmt.Errorf("failed to send start message: %w", err)
	}
	return nil
}

func showChatID(b *gotgbot.Bot, ctx *ext.Context) error {
	_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("%d", ctx.Message.Chat.Id), &gotgbot.SendMessageOpts{
		ParseMode: "html",
	})
	if err != nil {
		return fmt.Errorf("failed to send no nvidia-smi binary message: %w", err)
	}

	return nil
}

func state(b *gotgbot.Bot, ctx *ext.Context) error {
	if chatID != ctx.Message.Chat.Id {
		_, err := ctx.EffectiveMessage.Reply(b, "Sorry this bot is gated", &gotgbot.SendMessageOpts{
			ParseMode: "html",
		})
		if err != nil {
			return fmt.Errorf("failed to send no nvidia-smi binary message: %w", err)
		}

		return nil
	}

	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		_, err := ctx.EffectiveMessage.Reply(b, "No nvidia-smi binary", &gotgbot.SendMessageOpts{
			ParseMode: "html",
		})
		if err != nil {
			return fmt.Errorf("failed to send no nvidia-smi binary message: %w", err)
		}
	}

	cmd := exec.Command("nvidia-smi", "-q", "-x")

	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run nvidia-smi: %w", err)
	}
	if len(errb.String()) > 0 {
		return fmt.Errorf("nvidia-smi error: %s", errb.String())
	}

	var results NvidiaSmiLog
	err = xml.Unmarshal(outb.Bytes(), &results)
	if err != nil {
		return fmt.Errorf("failed to unmarshal xml: %w", err)
	}

	var info []string = []string{
		fmt.Sprintf("Timestamp: <b>%s</b>", results.Timestamp),
		fmt.Sprintf("Driver Version: <b>%s</b>", results.DriverVersion),
		fmt.Sprintf("CUDA Version: <b>%s</b>", results.CudaVersion),
		fmt.Sprintf("Attached GPUs: <b>%s</b>", results.AttachedGpus),
	}

	_, err = b.SendMessage(ctx.Message.Chat.Id, strings.Join(info, "\n"), &gotgbot.SendMessageOpts{
		ParseMode: "html",
	})
	if err != nil {
		return fmt.Errorf("failed to send a message: %w", err)
	}

	for _, gpuInfo := range results.Gpu {
		var fullGpuInfo []string = []string{
			fmt.Sprintf("GPU ID: <b>%s</b>", gpuInfo.ID),
			fmt.Sprintf("Product Name: <b>%s</b> (%s)", gpuInfo.ProductName, gpuInfo.ProductArchitecture),
			fmt.Sprintf("Fan speed: <b>%s</b>", gpuInfo.FanSpeed),
			"",
			fmt.Sprintf("Memory total: <b>%s</b>", gpuInfo.FbMemoryUsage.Total),
			fmt.Sprintf("Memory reserved: <b>%s</b>", gpuInfo.FbMemoryUsage.Reserved),
			fmt.Sprintf("Memory used: <b>%s</b>", gpuInfo.FbMemoryUsage.Used),
			fmt.Sprintf("Memory free: <b>%s</b>", gpuInfo.FbMemoryUsage.Free),
			"",
			fmt.Sprintf("GPU utilization: <b>%s</b>", gpuInfo.Utilization.GpuUtil),
			fmt.Sprintf("Memory utilization: <b>%s</b>", gpuInfo.Utilization.MemoryUtil),
			"",
			fmt.Sprintf("GPU temperature: <b>%s</b>", gpuInfo.Temperature.GpuTemp),
			// fmt.Sprintf("GPU temperature (max threshold): <b>%s</b>", gpuInfo.Temperature.GpuTempMaxThreshold),
			fmt.Sprintf("GPU power draw: <b>%s</b> / <b>%s</b>", gpuInfo.GpuPowerReadings.PowerDraw, gpuInfo.GpuPowerReadings.CurrentPowerLimit),
		}

		_, err = b.SendMessage(ctx.Message.Chat.Id, strings.Join(fullGpuInfo, "\n"), &gotgbot.SendMessageOpts{
			ParseMode: "html",
		})
		if err != nil {
			return fmt.Errorf("failed to send a message: %w", err)
		}
	}

	return nil
}

// NvidiaSmiLog was generated 2024-07-24 14:58:41 by https://xml-to-go.github.io/ in Ukraine.
type NvidiaSmiLog struct {
	XMLName       xml.Name `xml:"nvidia_smi_log"`
	Text          string   `xml:",chardata"`
	Timestamp     string   `xml:"timestamp"`
	DriverVersion string   `xml:"driver_version"`
	CudaVersion   string   `xml:"cuda_version"`
	AttachedGpus  string   `xml:"attached_gpus"`
	Gpu           []struct {
		Text                string `xml:",chardata"`
		ID                  string `xml:"id,attr"`
		ProductName         string `xml:"product_name"`
		ProductBrand        string `xml:"product_brand"`
		ProductArchitecture string `xml:"product_architecture"`
		DisplayMode         string `xml:"display_mode"`
		DisplayActive       string `xml:"display_active"`
		PersistenceMode     string `xml:"persistence_mode"`
		AddressingMode      string `xml:"addressing_mode"`
		MigMode             struct {
			Text       string `xml:",chardata"`
			CurrentMig string `xml:"current_mig"`
			PendingMig string `xml:"pending_mig"`
		} `xml:"mig_mode"`
		MigDevices               string `xml:"mig_devices"`
		AccountingMode           string `xml:"accounting_mode"`
		AccountingModeBufferSize string `xml:"accounting_mode_buffer_size"`
		DriverModel              struct {
			Text      string `xml:",chardata"`
			CurrentDm string `xml:"current_dm"`
			PendingDm string `xml:"pending_dm"`
		} `xml:"driver_model"`
		Serial           string `xml:"serial"`
		Uuid             string `xml:"uuid"`
		MinorNumber      string `xml:"minor_number"`
		VbiosVersion     string `xml:"vbios_version"`
		MultigpuBoard    string `xml:"multigpu_board"`
		BoardID          string `xml:"board_id"`
		BoardPartNumber  string `xml:"board_part_number"`
		GpuPartNumber    string `xml:"gpu_part_number"`
		GpuFruPartNumber string `xml:"gpu_fru_part_number"`
		GpuModuleID      string `xml:"gpu_module_id"`
		InforomVersion   struct {
			Text       string `xml:",chardata"`
			ImgVersion string `xml:"img_version"`
			OemObject  string `xml:"oem_object"`
			EccObject  string `xml:"ecc_object"`
			PwrObject  string `xml:"pwr_object"`
		} `xml:"inforom_version"`
		InforomBbxFlush struct {
			Text            string `xml:",chardata"`
			LatestTimestamp string `xml:"latest_timestamp"`
			LatestDuration  string `xml:"latest_duration"`
		} `xml:"inforom_bbx_flush"`
		GpuOperationMode struct {
			Text       string `xml:",chardata"`
			CurrentGom string `xml:"current_gom"`
			PendingGom string `xml:"pending_gom"`
		} `xml:"gpu_operation_mode"`
		C2cMode               string `xml:"c2c_mode"`
		GpuVirtualizationMode struct {
			Text                  string `xml:",chardata"`
			VirtualizationMode    string `xml:"virtualization_mode"`
			HostVgpuMode          string `xml:"host_vgpu_mode"`
			VgpuHeterogeneousMode string `xml:"vgpu_heterogeneous_mode"`
		} `xml:"gpu_virtualization_mode"`
		GpuResetStatus struct {
			Text                     string `xml:",chardata"`
			ResetRequired            string `xml:"reset_required"`
			DrainAndResetRecommended string `xml:"drain_and_reset_recommended"`
		} `xml:"gpu_reset_status"`
		GspFirmwareVersion string `xml:"gsp_firmware_version"`
		Ibmnpu             struct {
			Text                string `xml:",chardata"`
			RelaxedOrderingMode string `xml:"relaxed_ordering_mode"`
		} `xml:"ibmnpu"`
		Pci struct {
			Text           string `xml:",chardata"`
			PciBus         string `xml:"pci_bus"`
			PciDevice      string `xml:"pci_device"`
			PciDomain      string `xml:"pci_domain"`
			PciBaseClass   string `xml:"pci_base_class"`
			PciSubClass    string `xml:"pci_sub_class"`
			PciDeviceID    string `xml:"pci_device_id"`
			PciBusID       string `xml:"pci_bus_id"`
			PciSubSystemID string `xml:"pci_sub_system_id"`
			PciGpuLinkInfo struct {
				Text    string `xml:",chardata"`
				PcieGen struct {
					Text                 string `xml:",chardata"`
					MaxLinkGen           string `xml:"max_link_gen"`
					CurrentLinkGen       string `xml:"current_link_gen"`
					DeviceCurrentLinkGen string `xml:"device_current_link_gen"`
					MaxDeviceLinkGen     string `xml:"max_device_link_gen"`
					MaxHostLinkGen       string `xml:"max_host_link_gen"`
				} `xml:"pcie_gen"`
				LinkWidths struct {
					Text             string `xml:",chardata"`
					MaxLinkWidth     string `xml:"max_link_width"`
					CurrentLinkWidth string `xml:"current_link_width"`
				} `xml:"link_widths"`
			} `xml:"pci_gpu_link_info"`
			PciBridgeChip struct {
				Text           string `xml:",chardata"`
				BridgeChipType string `xml:"bridge_chip_type"`
				BridgeChipFw   string `xml:"bridge_chip_fw"`
			} `xml:"pci_bridge_chip"`
			ReplayCounter         string `xml:"replay_counter"`
			ReplayRolloverCounter string `xml:"replay_rollover_counter"`
			TxUtil                string `xml:"tx_util"`
			RxUtil                string `xml:"rx_util"`
			AtomicCapsInbound     string `xml:"atomic_caps_inbound"`
			AtomicCapsOutbound    string `xml:"atomic_caps_outbound"`
		} `xml:"pci"`
		FanSpeed           string `xml:"fan_speed"`
		PerformanceState   string `xml:"performance_state"`
		ClocksEventReasons struct {
			Text                                       string `xml:",chardata"`
			ClocksEventReasonGpuIdle                   string `xml:"clocks_event_reason_gpu_idle"`
			ClocksEventReasonApplicationsClocksSetting string `xml:"clocks_event_reason_applications_clocks_setting"`
			ClocksEventReasonSwPowerCap                string `xml:"clocks_event_reason_sw_power_cap"`
			ClocksEventReasonHwSlowdown                string `xml:"clocks_event_reason_hw_slowdown"`
			ClocksEventReasonHwThermalSlowdown         string `xml:"clocks_event_reason_hw_thermal_slowdown"`
			ClocksEventReasonHwPowerBrakeSlowdown      string `xml:"clocks_event_reason_hw_power_brake_slowdown"`
			ClocksEventReasonSyncBoost                 string `xml:"clocks_event_reason_sync_boost"`
			ClocksEventReasonSwThermalSlowdown         string `xml:"clocks_event_reason_sw_thermal_slowdown"`
			ClocksEventReasonDisplayClocksSetting      string `xml:"clocks_event_reason_display_clocks_setting"`
		} `xml:"clocks_event_reasons"`
		SparseOperationMode string `xml:"sparse_operation_mode"`
		FbMemoryUsage       struct {
			Text     string `xml:",chardata"`
			Total    string `xml:"total"`
			Reserved string `xml:"reserved"`
			Used     string `xml:"used"`
			Free     string `xml:"free"`
		} `xml:"fb_memory_usage"`
		Bar1MemoryUsage struct {
			Text  string `xml:",chardata"`
			Total string `xml:"total"`
			Used  string `xml:"used"`
			Free  string `xml:"free"`
		} `xml:"bar1_memory_usage"`
		CcProtectedMemoryUsage struct {
			Text  string `xml:",chardata"`
			Total string `xml:"total"`
			Used  string `xml:"used"`
			Free  string `xml:"free"`
		} `xml:"cc_protected_memory_usage"`
		ComputeMode string `xml:"compute_mode"`
		Utilization struct {
			Text        string `xml:",chardata"`
			GpuUtil     string `xml:"gpu_util"`
			MemoryUtil  string `xml:"memory_util"`
			EncoderUtil string `xml:"encoder_util"`
			DecoderUtil string `xml:"decoder_util"`
			JpegUtil    string `xml:"jpeg_util"`
			OfaUtil     string `xml:"ofa_util"`
		} `xml:"utilization"`
		EncoderStats struct {
			Text           string `xml:",chardata"`
			SessionCount   string `xml:"session_count"`
			AverageFps     string `xml:"average_fps"`
			AverageLatency string `xml:"average_latency"`
		} `xml:"encoder_stats"`
		FbcStats struct {
			Text           string `xml:",chardata"`
			SessionCount   string `xml:"session_count"`
			AverageFps     string `xml:"average_fps"`
			AverageLatency string `xml:"average_latency"`
		} `xml:"fbc_stats"`
		EccMode struct {
			Text       string `xml:",chardata"`
			CurrentEcc string `xml:"current_ecc"`
			PendingEcc string `xml:"pending_ecc"`
		} `xml:"ecc_mode"`
		EccErrors struct {
			Text     string `xml:",chardata"`
			Volatile struct {
				Text                    string `xml:",chardata"`
				SramCorrectable         string `xml:"sram_correctable"`
				SramUncorrectableParity string `xml:"sram_uncorrectable_parity"`
				SramUncorrectableSecded string `xml:"sram_uncorrectable_secded"`
				DramCorrectable         string `xml:"dram_correctable"`
				DramUncorrectable       string `xml:"dram_uncorrectable"`
			} `xml:"volatile"`
			Aggregate struct {
				Text                    string `xml:",chardata"`
				SramCorrectable         string `xml:"sram_correctable"`
				SramUncorrectableParity string `xml:"sram_uncorrectable_parity"`
				SramUncorrectableSecded string `xml:"sram_uncorrectable_secded"`
				DramCorrectable         string `xml:"dram_correctable"`
				DramUncorrectable       string `xml:"dram_uncorrectable"`
				SramThresholdExceeded   string `xml:"sram_threshold_exceeded"`
			} `xml:"aggregate"`
			AggregateUncorrectableSramSources struct {
				Text                string `xml:",chardata"`
				SramL2              string `xml:"sram_l2"`
				SramSm              string `xml:"sram_sm"`
				SramMicrocontroller string `xml:"sram_microcontroller"`
				SramPcie            string `xml:"sram_pcie"`
				SramOther           string `xml:"sram_other"`
			} `xml:"aggregate_uncorrectable_sram_sources"`
		} `xml:"ecc_errors"`
		RetiredPages struct {
			Text                        string `xml:",chardata"`
			MultipleSingleBitRetirement struct {
				Text            string `xml:",chardata"`
				RetiredCount    string `xml:"retired_count"`
				RetiredPagelist string `xml:"retired_pagelist"`
			} `xml:"multiple_single_bit_retirement"`
			DoubleBitRetirement struct {
				Text            string `xml:",chardata"`
				RetiredCount    string `xml:"retired_count"`
				RetiredPagelist string `xml:"retired_pagelist"`
			} `xml:"double_bit_retirement"`
			PendingBlacklist  string `xml:"pending_blacklist"`
			PendingRetirement string `xml:"pending_retirement"`
		} `xml:"retired_pages"`
		RemappedRows struct {
			Text                 string `xml:",chardata"`
			RemappedRowCorr      string `xml:"remapped_row_corr"`
			RemappedRowUnc       string `xml:"remapped_row_unc"`
			RemappedRowPending   string `xml:"remapped_row_pending"`
			RemappedRowFailure   string `xml:"remapped_row_failure"`
			RowRemapperHistogram struct {
				Text                        string `xml:",chardata"`
				RowRemapperHistogramMax     string `xml:"row_remapper_histogram_max"`
				RowRemapperHistogramHigh    string `xml:"row_remapper_histogram_high"`
				RowRemapperHistogramPartial string `xml:"row_remapper_histogram_partial"`
				RowRemapperHistogramLow     string `xml:"row_remapper_histogram_low"`
				RowRemapperHistogramNone    string `xml:"row_remapper_histogram_none"`
			} `xml:"row_remapper_histogram"`
		} `xml:"remapped_rows"`
		Temperature struct {
			Text                   string `xml:",chardata"`
			GpuTemp                string `xml:"gpu_temp"`
			GpuTempTlimit          string `xml:"gpu_temp_tlimit"`
			GpuTempMaxThreshold    string `xml:"gpu_temp_max_threshold"`
			GpuTempSlowThreshold   string `xml:"gpu_temp_slow_threshold"`
			GpuTempMaxGpuThreshold string `xml:"gpu_temp_max_gpu_threshold"`
			GpuTargetTemperature   string `xml:"gpu_target_temperature"`
			MemoryTemp             string `xml:"memory_temp"`
			GpuTempMaxMemThreshold string `xml:"gpu_temp_max_mem_threshold"`
		} `xml:"temperature"`
		SupportedGpuTargetTemp struct {
			Text             string `xml:",chardata"`
			GpuTargetTempMin string `xml:"gpu_target_temp_min"`
			GpuTargetTempMax string `xml:"gpu_target_temp_max"`
		} `xml:"supported_gpu_target_temp"`
		GpuPowerReadings struct {
			Text                string `xml:",chardata"`
			PowerState          string `xml:"power_state"`
			PowerDraw           string `xml:"power_draw"`
			CurrentPowerLimit   string `xml:"current_power_limit"`
			RequestedPowerLimit string `xml:"requested_power_limit"`
			DefaultPowerLimit   string `xml:"default_power_limit"`
			MinPowerLimit       string `xml:"min_power_limit"`
			MaxPowerLimit       string `xml:"max_power_limit"`
		} `xml:"gpu_power_readings"`
		GpuMemoryPowerReadings struct {
			Text      string `xml:",chardata"`
			PowerDraw string `xml:"power_draw"`
		} `xml:"gpu_memory_power_readings"`
		ModulePowerReadings struct {
			Text                string `xml:",chardata"`
			PowerState          string `xml:"power_state"`
			PowerDraw           string `xml:"power_draw"`
			CurrentPowerLimit   string `xml:"current_power_limit"`
			RequestedPowerLimit string `xml:"requested_power_limit"`
			DefaultPowerLimit   string `xml:"default_power_limit"`
			MinPowerLimit       string `xml:"min_power_limit"`
			MaxPowerLimit       string `xml:"max_power_limit"`
		} `xml:"module_power_readings"`
		Clocks struct {
			Text          string `xml:",chardata"`
			GraphicsClock string `xml:"graphics_clock"`
			SmClock       string `xml:"sm_clock"`
			MemClock      string `xml:"mem_clock"`
			VideoClock    string `xml:"video_clock"`
		} `xml:"clocks"`
		ApplicationsClocks struct {
			Text          string `xml:",chardata"`
			GraphicsClock string `xml:"graphics_clock"`
			MemClock      string `xml:"mem_clock"`
		} `xml:"applications_clocks"`
		DefaultApplicationsClocks struct {
			Text          string `xml:",chardata"`
			GraphicsClock string `xml:"graphics_clock"`
			MemClock      string `xml:"mem_clock"`
		} `xml:"default_applications_clocks"`
		DeferredClocks struct {
			Text     string `xml:",chardata"`
			MemClock string `xml:"mem_clock"`
		} `xml:"deferred_clocks"`
		MaxClocks struct {
			Text          string `xml:",chardata"`
			GraphicsClock string `xml:"graphics_clock"`
			SmClock       string `xml:"sm_clock"`
			MemClock      string `xml:"mem_clock"`
			VideoClock    string `xml:"video_clock"`
		} `xml:"max_clocks"`
		MaxCustomerBoostClocks struct {
			Text          string `xml:",chardata"`
			GraphicsClock string `xml:"graphics_clock"`
		} `xml:"max_customer_boost_clocks"`
		ClockPolicy struct {
			Text             string `xml:",chardata"`
			AutoBoost        string `xml:"auto_boost"`
			AutoBoostDefault string `xml:"auto_boost_default"`
		} `xml:"clock_policy"`
		Voltage struct {
			Text         string `xml:",chardata"`
			GraphicsVolt string `xml:"graphics_volt"`
		} `xml:"voltage"`
		Fabric struct {
			Text        string `xml:",chardata"`
			State       string `xml:"state"`
			Status      string `xml:"status"`
			CliqueId    string `xml:"cliqueId"`
			ClusterUuid string `xml:"clusterUuid"`
			Health      struct {
				Text      string `xml:",chardata"`
				Bandwidth string `xml:"bandwidth"`
			} `xml:"health"`
		} `xml:"fabric"`
		SupportedClocks struct {
			Text              string `xml:",chardata"`
			SupportedMemClock []struct {
				Text                   string   `xml:",chardata"`
				Value                  string   `xml:"value"`
				SupportedGraphicsClock []string `xml:"supported_graphics_clock"`
			} `xml:"supported_mem_clock"`
		} `xml:"supported_clocks"`
		Processes struct {
			Text        string `xml:",chardata"`
			ProcessInfo []struct {
				Text              string `xml:",chardata"`
				GpuInstanceID     string `xml:"gpu_instance_id"`
				ComputeInstanceID string `xml:"compute_instance_id"`
				Pid               string `xml:"pid"`
				Type              string `xml:"type"`
				ProcessName       string `xml:"process_name"`
				UsedMemory        string `xml:"used_memory"`
			} `xml:"process_info"`
		} `xml:"processes"`
		AccountedProcesses string `xml:"accounted_processes"`
		Capabilities       struct {
			Text string `xml:",chardata"`
			Egm  string `xml:"egm"`
		} `xml:"capabilities"`
	} `xml:"gpu"`
}
