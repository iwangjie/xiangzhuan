use nsh_core::prelude::*;

#[native_module("xiangzhuan/windows")]
fn native(registry: &mut Registry) {
    registry.module::<InputActivity>();
}

#[module]
pub struct InputActivity;

#[module]
impl InputActivity {
    /// Milliseconds since the last keyboard or mouse input in this session.
    pub fn idle_milliseconds() -> Option<f64> {
        idle_milliseconds()
    }
}

#[cfg(windows)]
fn idle_milliseconds() -> Option<f64> {
    #[repr(C)]
    struct LastInputInfo {
        size: u32,
        tick: u32,
    }

    #[link(name = "user32")]
    unsafe extern "system" {
        fn GetLastInputInfo(info: *mut LastInputInfo) -> i32;
    }

    #[link(name = "kernel32")]
    unsafe extern "system" {
        fn GetTickCount() -> u32;
    }

    let mut info = LastInputInfo {
        size: size_of::<LastInputInfo>() as u32,
        tick: 0,
    };
    // The struct has the layout and cbSize required by GetLastInputInfo.
    if unsafe { GetLastInputInfo(&mut info) } == 0 {
        return None;
    }
    let elapsed = unsafe { GetTickCount() }.wrapping_sub(info.tick);
    // A tick reported slightly ahead of GetTickCount means recent input.
    Some(f64::from(if elapsed <= i32::MAX as u32 {
        elapsed
    } else {
        0
    }))
}

#[cfg(not(windows))]
fn idle_milliseconds() -> Option<f64> {
    None
}
