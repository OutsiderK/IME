//
//    Copyright (C) 2013 - 2020 Hong Jen Yee (PCMan) <pcman.tw@gmail.com>
//
//    This library is free software; you can redistribute it and/or
//    modify it under the terms of the GNU Library General Public
//    License as published by the Free Software Foundation; either
//    version 2 of the License, or (at your option) any later version.
//
//    This library is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
//    Library General Public License for more details.
//
//    You should have received a copy of the GNU Library General Public
//    License along with this library; if not, write to the
//    Free Software Foundation, Inc., 51 Franklin St, Fifth Floor,
//    Boston, MA  02110-1301, USA.
//

#include "KeyEvent.h"

namespace Ime {

KeyEvent::KeyEvent(UINT type, WPARAM wp, LPARAM lp):
    type_(type),
    keyCode_(wp),
    lParam_(lp) {

    if(!::GetKeyboardState(keyStates_)) // get state of all keys
        ::memset(keyStates_, 0, sizeof(keyStates_));

    // Convert the virtual key to a printable character in the active layout.
    // Clear Ctrl/Alt so shortcuts still yield the underlying printable char.
    WCHAR result[8] = {0};
    BYTE translatedStates[256];
    ::memcpy(translatedStates, keyStates_, sizeof(translatedStates));
    translatedStates[VK_CONTROL] = 0;
    translatedStates[VK_MENU] = 0;
    const int translated = ::ToUnicodeEx(keyCode_, scanCode(), translatedStates,
                                         result, ARRAYSIZE(result), 0,
                                         ::GetKeyboardLayout(0));
    if(translated == 1)
        charCode_ = (UINT)result[0];
    else
        charCode_ = 0;
}

KeyEvent::~KeyEvent(void) {
}

} // namespace Ime
