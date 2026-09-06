# Shared Storage Options for regattaClock

An evaluation of running a local NAS as the shared `regattaData` location, with OneDrive relegated to off-site backup. This is exploratory and deliberately separate from [persona-plan.md](persona-plan.md), which assumes OneDrive; nothing here is committed to.

## Short answer

Yes, a NAS or spare Windows PC can serve SMB to the local personas and act as a OneDrive client at the same time — every major vendor supports it, and so does a cheap Windows mini-PC. And yes, it genuinely fixes the problem you are worried about, because it removes the internet from the live path entirely.

**Status relative to the persona plan:** [persona-plan.md](persona-plan.md) now treats OneDrive vs local SMB as a first-class `PrefStorageMode` configuration (`onedrive` | `smb`), with LAN NTP preferred under SMB. This document remains the deeper rationale and ops guide; the plan is what the app will implement.

The catch is not the NAS. It is that a rowing course puts the Start Timer and the Finish Timer 1500-2000m apart, and "all persona laptops are on the same network" is the assumption doing the heavy lifting. See section 6.

## 1. Your diagnosis is correct

Worth stating explicitly, because it is the whole justification: **OneDrive's local mirror does not help peer-to-peer.** Each laptop keeps a full local copy and can keep working offline, but propagation from one laptop to another goes up to Microsoft and back down. OneDrive has no LAN-direct sync. So if the venue's internet drops:

- Each persona keeps collecting into their own local mirror. Nothing is lost.
- No persona sees anyone else's data until connectivity returns.
- Concretely: the Finish Timer stops receiving start times, and every winning time falls back to manual entry for the duration.

That is exactly the failure the staleness indicator in the persona plan exists to make visible. A NAS makes it not happen in the first place.

## 2. Can a NAS be both an SMB server and a OneDrive client?

Yes, two viable shapes.

**A NAS appliance with a cloud-sync package.** Synology's Cloud Sync, QNAP's Hybrid Backup Sync, and TrueNAS's rclone-based cloud sync tasks all support OneDrive and OneDrive for Business. These talk to Microsoft Graph rather than running Microsoft's client, which has one consequence worth checking early: **OneDrive for Business tenants frequently apply Conditional Access policies that block third-party API clients.** If your organization's IT has done that, the NAS will authenticate and then fail, and you will find out at the worst possible moment. Confirm this before buying hardware.

**A Windows mini-PC running the genuine OneDrive client, sharing a folder over SMB.** Less elegant, but it sidesteps the Conditional Access problem entirely because it *is* the sanctioned client. It is also cheaper than most NAS units. Two caveats: set the shared folder to "Always keep on this device" so Files On-Demand does not try to hydrate files when a laptop reads them over SMB, and accept that you are maintaining a Windows box.

Either way, **make the cloud sync one-directional, NAS to OneDrive.** Bidirectional sync means a cloud-side change can propagate back down and overwrite the NAS mid-regatta. The cloud copy should be a backup you can restore from, never an input.

## 3. What this changes in the application

The app itself barely changes — it takes a directory path, and `\\nas\regatta\regattaData` is a directory path. Use the UNC form rather than a mapped drive letter; drive letters are per-user and have a habit of failing to reconnect after a laptop wakes. Go handles UNC paths on Windows without special treatment, and `filepath.Base` behaves normally on them.

What does change:

- **Sync latency drops from seconds to milliseconds.** The reactive winning-time design in the persona plan stays correct and useful, but it stops being the common case. The Finish Timer would usually have the start time before the race is over.
- **Conflict copies stop existing.** SMB is a real filesystem, not a sync engine; simultaneous writes produce a sharing violation rather than a `-DESKTOP-A1B2C3` duplicate. The single-writer-per-file model still earns its keep, and the conflict-copy *detection* becomes dead code.
- **Atomic writes get more reliable but still need the retry loop.** The OneDrive sync engine is no longer grabbing handles, but SMB has its own sharing-violation semantics and Defender is still local. Keep the retry.
- **The watcher needs re-testing, and may need to change strategy.** This one is subtle and worth calling out.

### The SMB client cache gotcha

The Windows SMB client caches file metadata. `FileInfoCacheLifetime` and `DirectoryCacheLifetime` under `HKLM\SYSTEM\CurrentControlSet\Services\LanmanWorkstation\Parameters` both default to roughly 10 seconds. That means a polling watcher calling `os.Stat` more often than every 10 seconds may keep getting a cached modification time and miss the change it is looking for — the exact opposite of the speedup you would expect from moving to a LAN.

Two ways out, and this needs measuring rather than guessing:

- Set those cache lifetimes to 0 on each laptop. Effective, but it is a registry change on managed machines and adds SMB round-trips.
- Switch to `fsnotify` for this deployment. Windows implements it via `ReadDirectoryChangesW`, which SMB2 servers support as a server-pushed change notification, bypassing the client cache. This is interesting because it inverts the recommendation in the persona plan: polling wins on OneDrive precisely because of Files On-Demand, and change notification may win on SMB precisely because of metadata caching.

If you go the NAS route, plan on the watcher supporting both strategies behind one interface, chosen by configuration.

## 4. The best argument for a NAS is not storage

**Run an NTP server on it.** Synology, QNAP, and TrueNAS can all act as a LAN NTP server, and this directly solves the fallback case in section 2.1 of the persona plan, where UDP 123 is blocked or the venue has no internet and the app cannot measure its clock offset at all.

It works even with the internet completely down, because of the point that section makes: the clocks do not need to be *correct*, they need to share a time base. Four laptops synced to a NAS that is itself drifting from UTC still produce perfectly accurate race times, because every offset is measured against the same reference. Point `internal/timesync` at the NAS address and the entire clock-skew problem becomes a solved, local, offline-capable concern.

That is a stronger reason to do this than the file sharing is.

## 5. The honest downside: you are trading one failure mode for another

With OneDrive, an internet outage degrades to **isolated but fully functional** — every persona keeps collecting into a complete local mirror and nothing is lost, they just cannot see each other.

With a NAS, an outage of the NAS, the switch, or the access point degrades to **cannot write at all**, and it hits all four personas simultaneously. That is a worse worst case. The NAS is a single point of failure that OneDrive's architecture specifically does not have.

The mitigation is at the application level and is a good idea either way: **a local write-ahead journal.** Have the app write every collected value to a local file first, then to the shared location. If the shared location is unreachable, keep collecting locally and flush when it returns. This makes the app robust to *any* storage outage rather than to a particular one, and it is a meaningful amount of work — worth costing out before committing to hardware that only helps if you also build it.

## 6. The thing to settle first: can the start and finish lines actually share a network?

A standard rowing course is 1500-2000m. The Start Timer is at one end and the Finish Timer at the other. "All persona laptops on the same network" is not a small assumption at that distance, and it may be the reason a cloud service was the natural first answer: each end has its own cellular connection and the cloud bridges them.

Three ways this could hold:

- **The venue already has course-wide WiFi.** Some clubs do. If so, this is easy and the NAS makes obvious sense.
- **A point-to-point wireless bridge between the two ends.** A rowing course is close to the ideal case for this — dead straight, unobstructed line of sight over open water. A pair of Ubiquiti airMAX-class radios covers 2km comfortably for a couple hundred dollars, and would carry both the SMB traffic and NTP. This is the option I would actually investigate.
- **Only the finish line has coverage.** Then the NAS helps the Finish Timer and the Director but not the Start Timer, which is precisely backwards — the start times are the data that has to travel.

If none of these hold, the NAS does not solve the problem and OneDrive remains the right answer.

## 7. A cheaper thing to try first

Before buying anything: **share the folder from one of the laptops you already bring.** The Regatta Director's machine is already at the venue, already powered, and is the natural host since the RD is read-only during racing. It is a Windows SMB share and a OneDrive client on the same box, which is exactly the mini-PC option without the mini-PC.

This costs nothing, tests every assumption in this document — the network coverage, the SMB cache behaviour, the watcher strategy, the local NTP — and if it works you can decide whether dedicated hardware is worth it. If it does not work, you have learned that for free.

The obvious weakness is that the host laptop becomes critical and its owner has to not close the lid. For a proof of concept that is acceptable.

## 8. Recommendation

Ordered by what I would actually do:

1. **Answer the network question first.** Everything else is contingent on it, and it is the one thing no amount of software design can work around.
2. **Prototype with a laptop share**, not hardware. Measure the SMB metadata cache behaviour against the watcher specifically — that is the one technical unknown that could sink the design quietly.
3. **Adopt the LAN NTP server regardless of how the storage question lands.** It is cheap, it is independent of the file-sharing decision, and it removes the largest correctness risk in the whole feature.
4. **Build the local write-ahead journal regardless.** It is the mitigation that makes the app robust to storage failure in general, and it is the only item here that improves the OneDrive deployment too.
5. **Only then decide on a NAS**, with one-way sync up to OneDrive for off-site backup.

## 9. Worth knowing for later

If this system outgrows files-on-a-share, the natural next step is not a better filesystem — it is a small HTTP or WebSocket service that the personas talk to, with the shared directory as its persistence layer. That gets you real push updates, proper conflict semantics, and no dependence on the sync behaviour of whatever storage is underneath. It is a significantly larger change and the file-based design in [persona.md](persona.md) is a deliberate, reasonable choice for the current scale, so this is a note for the future rather than a suggestion for now.
