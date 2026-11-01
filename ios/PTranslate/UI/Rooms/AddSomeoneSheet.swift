import CoreImage.CIFilterBuiltins
import SwiftUI
import Vision
import VisionKit

/// Adding someone: by their ID, or by scanning their QR code, with the user's own code to show.
struct AddSomeoneSheet: View {
    let viewModel: RoomsViewModel
    let me: Profile
    let onClose: () -> Void

    private enum Tab { case byID, qr }

    @State private var tab = Tab.byID
    @State private var scanning = false
    @State private var scanProblem: String?

    var body: some View {
        NavigationStack {
            VStack(spacing: 20) {
                Picker("Add by", selection: $tab) {
                    Text("By ID").tag(Tab.byID)
                    Text("QR code").tag(Tab.qr)
                }
                .pickerStyle(.segmented)
                switch tab {
                case .byID: byID
                case .qr: myCode
                }
                Spacer(minLength: 0)
            }
            .padding(.horizontal, 16)
            .padding(.top, 8)
            .background(Palette.bg)
            .navigationTitle("Add someone")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button(action: onClose) { Image(systemName: "xmark") }.accessibilityLabel("Close")
                }
            }
            .fullScreenCover(isPresented: $scanning) {
                CodeScanner { raw in
                    scanning = false
                    if let handle = handle(fromCode: raw) {
                        tab = .byID
                        viewModel.lookupChanged(handle)
                    } else {
                        scanProblem = "That code isn't a \(Brand.name) ID. Ask them to open Add someone, then QR code."
                    }
                } onCancel: {
                    scanning = false
                }
                .ignoresSafeArea()
            }
        }
    }

    @ViewBuilder
    private var byID: some View {
        PearlTextField(label: "Their ID", text: Binding(get: { viewModel.lookupHandle }, set: { viewModel.lookupChanged($0) }), placeholder: "their.id", prefix: "@") {
            if viewModel.lookup == .searching { ProgressView().tint(Palette.fg2) }
        }
        .textInputAutocapitalization(.never)
        .autocorrectionDisabled()
        .keyboardType(.asciiCapable)
        switch viewModel.lookup {
        case .found(let person): PersonCard(person: person, me: me, opening: viewModel.opening, onAdd: viewModel.addTapped)
        case .notFound(let reason): Notice(text: reason).frame(maxWidth: .infinity, alignment: .leading)
        case .idle, .searching: EmptyView()
        }
    }

    private var myCode: some View {
        VStack(spacing: 8) {
            QRCodeImage(text: addLink(me.handle).absoluteString)
                .padding(18)
                .frame(width: 236, height: 236)
                .background(Palette.you, in: RoundedRectangle(cornerRadius: 20))
                .accessibilityLabel("Your QR code, for @\(me.handle)")
            Text(me.displayName).font(.pearlTitle).foregroundStyle(Palette.fg).padding(.top, 16)
            Text("@\(me.handle) · speaks \(me.lang?.localName ?? "")").font(.pearlMessage).foregroundStyle(Palette.fg2)
            Text("Let them scan this with their phone's camera, or the app.")
                .font(.pearlMessage).foregroundStyle(Palette.fg2)
                .multilineTextAlignment(.center).padding(.top, 8)
            if let scanProblem {
                Notice(text: scanProblem).padding(.top, 4)
            }
            Button {
                scanProblem = nil
                if DataScannerViewController.isSupported && DataScannerViewController.isAvailable {
                    scanning = true
                } else {
                    scanProblem = "The camera can't scan here. Allow camera access in Settings, or add them by ID."
                }
            } label: {
                Label("Scan their code", systemImage: "qrcode.viewfinder")
            }
            .buttonStyle(PrimaryButtonStyle())
            .padding(.top, 16)
            ShareLink(item: addLink(me.handle), message: Text(shareMessage(me.handle))) {
                Text("Share my code")
            }
            .buttonStyle(SecondaryButtonStyle())
        }
    }
}

/// Who was found, what each will hear, and the one action.
private struct PersonCard: View {
    let person: Profile
    let me: Profile
    let opening: Bool
    let onAdd: () -> Void

    var body: some View {
        let first = person.displayName.split(separator: " ").first.map(String.init) ?? "@\(person.handle)"
        VStack(alignment: .leading, spacing: 16) {
            HStack(spacing: 16) {
                Avatar(name: person.shownName, size: 52, fill: Palette.bg)
                VStack(alignment: .leading, spacing: 2) {
                    Text(person.shownName).font(.custom("IBMPlexSans-Medm", size: 18, relativeTo: .title3)).foregroundStyle(Palette.fg)
                    let language = Text(person.lang?.ownName ?? "").foregroundStyle(Palette.them)
                    Text("@\(person.handle) · speaks \(language)").font(.pearlBody).foregroundStyle(Palette.fg2)
                }
            }
            // Pronouns are not known, so the sentence is written around them.
            Text("You'll hear \(first) in \(me.lang?.localName ?? ""), in their own voice. They'll hear you in \(person.lang?.localName ?? ""), in yours.")
                .font(.pearlMessage).foregroundStyle(Palette.fg2)
            Button(opening ? "Opening…" : "Add \(first)", action: onAdd)
                .buttonStyle(PrimaryButtonStyle())
                .disabled(opening)
        }
        .padding(16)
        .background(Palette.bgRaised, in: RoundedRectangle(cornerRadius: 12))
    }
}

/// A QR code of `text`, from Core Image, drawn crisp: dark modules on the light tile, as scanners expect.
private struct QRCodeImage: View {
    let text: String

    var body: some View {
        if let image = Self.render(text) {
            Image(decorative: image, scale: 1)
                .interpolation(.none)
                .resizable()
                .scaledToFit()
                .colorMultiply(Palette.onYou)
        }
    }

    private static func render(_ text: String) -> CGImage? {
        let filter = CIFilter.qrCodeGenerator()
        filter.message = Data(text.utf8)
        filter.correctionLevel = "M"
        guard let output = filter.outputImage else { return nil }
        return CIContext().createCGImage(output, from: output.extent)
    }
}

/// VisionKit's live scanner, set to QR codes; it asks for the camera itself.
private struct CodeScanner: UIViewControllerRepresentable {
    let onCode: (String) -> Void
    let onCancel: () -> Void

    func makeUIViewController(context: Context) -> UINavigationController {
        let scanner = DataScannerViewController(
            recognizedDataTypes: [.barcode(symbologies: [.qr])],
            isHighlightingEnabled: true
        )
        scanner.delegate = context.coordinator
        scanner.navigationItem.leftBarButtonItem = UIBarButtonItem(
            systemItem: .cancel,
            primaryAction: UIAction { _ in onCancel() }
        )
        try? scanner.startScanning()
        return UINavigationController(rootViewController: scanner)
    }

    func updateUIViewController(_ controller: UINavigationController, context: Context) {}

    func makeCoordinator() -> Coordinator {
        Coordinator(onCode: onCode)
    }

    final class Coordinator: NSObject, DataScannerViewControllerDelegate {
        let onCode: (String) -> Void
        private var done = false

        init(onCode: @escaping (String) -> Void) {
            self.onCode = onCode
        }

        func dataScanner(_ scanner: DataScannerViewController, didAdd items: [RecognizedItem], allItems: [RecognizedItem]) {
            guard !done else { return }
            for case .barcode(let code) in items {
                if let raw = code.payloadStringValue {
                    done = true
                    scanner.stopScanning()
                    onCode(raw)
                    return
                }
            }
        }
    }
}
