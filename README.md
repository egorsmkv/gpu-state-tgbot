# `gpu-state-tgbot`

This little bot shows info about your GPUs. It uses `nvidia-smi` to get the info.

## Installation

```shell
go build -o tg-state-bot main.go
```

## Example 

```
Timestamp: Wed Jul 24 15:34:38 2024
Driver Version: 555.42.06
CUDA Version: 12.5
Attached GPUs: 2

---

GPU ID: 00000000:02:00.0
Product Name: NVIDIA RTX A4000 (Ampere)
Fan speed: 88 %

Memory total: 16376 MiB
Memory reserved: 366 MiB
Memory used: 14551 MiB
Memory free: 1460 MiB

GPU utilization: 39 %
Memory utilization: 42 %

GPU temperature: 93 C
GPU power draw: 124.19 W / 140.00 W

---

GPU ID: 00000000:03:00.0
Product Name: NVIDIA RTX A4000 (Ampere)
Fan speed: 100 %

Memory total: 16376 MiB
Memory reserved: 365 MiB
Memory used: 14501 MiB
Memory free: 1512 MiB

GPU utilization: 45 %
Memory utilization: 24 %

GPU temperature: 95 C
GPU power draw: 121.41 W / 140.00 W
```
