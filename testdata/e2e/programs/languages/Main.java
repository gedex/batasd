public class Main {
    public static void main(String[] args) throws Exception {
        byte[] data = System.in.readAllBytes();
        String name = new String(data).trim();
        System.out.println("hello from java: " + name);
    }
}
