package com.example.ptranslate.ui.rooms

import android.content.Context
import android.content.Intent
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.example.ptranslate.R
import com.example.ptranslate.core.account.Profile
import com.example.ptranslate.core.rooms.Room
import com.example.ptranslate.ui.components.Avatar
import com.example.ptranslate.ui.components.Notice
import com.example.ptranslate.ui.components.PearlIcons
import com.example.ptranslate.ui.components.PearlTextField
import com.example.ptranslate.ui.components.PrimaryButton
import com.example.ptranslate.ui.components.SecondaryButton
import com.example.ptranslate.ui.components.Toast
import com.example.ptranslate.ui.components.TwoVoices
import com.example.ptranslate.ui.components.localName
import com.example.ptranslate.ui.components.ownName
import com.example.ptranslate.ui.theme.Pearl
import com.example.ptranslate.ui.theme.PearlType

/**
 * The people the user talks with. [addHandle] is an ID from a link
 * (shinar://add/...) to look up in the "Add someone" sheet straight away.
 */
@Composable
fun RoomsScreen(
    viewModel: RoomsViewModel,
    onOpenRoom: (Room) -> Unit,
    modifier: Modifier = Modifier,
    addHandle: String? = null,
    onAddHandleUsed: () -> Unit = {},
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val me = viewModel.me
    val context = LocalContext.current
    var adding by rememberSaveable { mutableStateOf(false) }

    LaunchedEffect(addHandle) {
        if (addHandle != null) {
            adding = true
            viewModel.onLookupChanged(addHandle)
            onAddHandleUsed()
        }
    }
    LaunchedEffect(state.opened) {
        state.opened?.let {
            adding = false
            onOpenRoom(it)
            viewModel.onOpenedHandled()
        }
    }

    Box(modifier.fillMaxSize()) {
        if (!state.loading && state.rooms.isEmpty()) {
            EmptyRooms(me, onAdd = { adding = true }, onShare = { me?.let { shareId(context, it) } })
        } else {
            RoomList(state, me, onAdd = { adding = true }, onQueryChange = viewModel::onQueryChanged, onOpenRoom = onOpenRoom)
        }
        if (!state.connected) {
            Toast("Connection lost. Reconnecting…", Modifier.align(Alignment.TopCenter).padding(top = 8.dp))
        }
    }

    if (adding && me != null) {
        AddSomeoneSheet(
            state = state,
            me = me,
            onHandleChange = viewModel::onLookupChanged,
            onAdd = viewModel::onAddClicked,
            onShare = { shareId(context, me) },
            onDismiss = {
                adding = false
                viewModel.onSheetClosed()
            },
        )
    }
}

@Composable
private fun RoomList(
    state: RoomsUiState,
    me: Profile?,
    onAdd: () -> Unit,
    onQueryChange: (String) -> Unit,
    onOpenRoom: (Room) -> Unit,
) {
    val colors = Pearl.colors
    LazyColumn(Modifier.fillMaxSize(), contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 16.dp, vertical = 8.dp)) {
        item {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                Box(
                    Modifier
                        .size(44.dp)
                        .clip(CircleShape)
                        .background(colors.bgRaised)
                        .clickable(role = Role.Button, onClickLabel = "Add someone", onClick = onAdd)
                        .semantics { contentDescription = "Add someone" },
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(PearlIcons.Plus, contentDescription = null, tint = colors.fg, modifier = Modifier.size(22.dp))
                }
            }
        }
        item {
            Text("Rooms", style = PearlType.largeTitle, color = colors.fg, modifier = Modifier.padding(vertical = 8.dp).semantics { heading() })
        }
        item {
            PearlTextField(
                value = state.query,
                onValueChange = onQueryChange,
                label = "Search rooms",
                placeholder = "Search",
                leading = PearlIcons.Search,
                compact = true,
                modifier = Modifier.padding(bottom = 16.dp),
            )
        }
        state.problem?.let { item { Notice(it, Modifier.padding(bottom = 12.dp)) } }
        items(state.shown, key = { it.id }) { room ->
            RoomRow(room, me, onClick = { onOpenRoom(room) })
        }
    }
}

@Composable
private fun RoomRow(room: Room, me: Profile?, onClick: () -> Unit) {
    val colors = Pearl.colors
    val other = room.other
    val name = other?.displayName?.ifBlank { null } ?: other?.handle?.let { "@$it" } ?: "Deleted account"
    Row(
        Modifier
            .fillMaxWidth()
            .clickable(role = Role.Button, onClick = onClick)
            .padding(vertical = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Avatar(name)
        Column(Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(name, style = PearlType.messageStrong, color = colors.fg, modifier = Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(roomTime(room.lastMessage?.createdAt ?: room.createdAt), style = PearlType.meta, color = colors.fg2)
            }
            Row(verticalAlignment = Alignment.Top) {
                Text(
                    room.lastMessage?.preview(me?.language) ?: "No messages yet",
                    style = PearlType.body.copy(fontSize = PearlType.message.fontSize.times(0.94f)),
                    color = colors.fg.copy(alpha = 0.72f),
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f).padding(top = 2.dp),
                )
                if (room.unread > 0) {
                    Box(
                        Modifier
                            .padding(start = 8.dp, top = 6.dp)
                            .size(10.dp)
                            .clip(CircleShape)
                            .background(colors.them)
                            .semantics { contentDescription = "${room.unread} new" },
                    )
                }
            }
            other?.lang?.let {
                Text("hears ${it.ownName()}", style = PearlType.meta, color = colors.them, modifier = Modifier.padding(top = 4.dp))
            }
            HorizontalDivider(color = colors.fg.copy(alpha = 0.08f), modifier = Modifier.padding(top = 14.dp))
        }
    }
}

@Composable
private fun EmptyRooms(me: Profile?, onAdd: () -> Unit, onShare: () -> Unit) {
    val colors = Pearl.colors
    Column(Modifier.fillMaxSize().padding(horizontal = 16.dp, vertical = 8.dp)) {
        Spacer(Modifier.size(52.dp))
        Text("Rooms", style = PearlType.largeTitle, color = colors.fg, modifier = Modifier.semantics { heading() })
        Column(
            Modifier.weight(1f).fillMaxWidth().padding(horizontal = 12.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
        ) {
            TwoVoices()
            Text("No one here yet.", style = PearlType.title, color = colors.fg, modifier = Modifier.padding(top = 28.dp))
            Text(
                "Add someone by their ID or QR code. You'll hear them in ${me?.lang?.localName() ?: "your language"}, in their own voice.",
                style = PearlType.message.copy(fontSize = PearlType.message.fontSize.times(1.06f)),
                color = colors.fg2,
                textAlign = TextAlign.Center,
                modifier = Modifier.padding(top = 10.dp, bottom = 24.dp),
            )
            PrimaryButton("Add someone", onClick = onAdd)
            if (me != null && me.handle.isNotEmpty()) {
                SecondaryButton("Share my ID, @${me.handle}", onClick = onShare, modifier = Modifier.padding(top = 12.dp))
            }
        }
    }
}

/** The start of the link in a person's QR code and in what they share. The scheme is in the manifest too. */
internal const val ADD_LINK_PREFIX = "shinar://add/"

/** The link in a person's QR code and in what they share: opening it offers to add them. */
fun addLink(handle: String) = ADD_LINK_PREFIX + handle

/** Shares the user's ID through Android's share sheet. */
fun shareId(context: Context, me: Profile) {
    val text = "Talk to me on ${context.getString(R.string.app_name)}, in your own language: my ID is @${me.handle}\n${addLink(me.handle)}"
    val send = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, text)
    context.startActivity(Intent.createChooser(send, "Share my ID"))
}
