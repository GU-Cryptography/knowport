import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App.tsx";
import AuthGate from "./components/AuthGate";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <AuthGate>
      {(user, logout) => (
        <App key={user.id} authUser={user} onLogout={logout} />
      )}
    </AuthGate>
  </React.StrictMode>,
);
